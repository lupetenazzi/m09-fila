package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Telemetry struct {
	DeviceID   string    `json:"device_id"`
	Timestamp  time.Time `json:"timestamp"`
	SensorType string    `json:"sensor_type"`
	ValueType  string    `json:"value_type"`
	Value      float64   `json:"value"`
}

func connectPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

func getOrCreateDevice(db *sql.DB, identifier string) (int, error) {
	var id int
	err := db.QueryRow("SELECT id FROM devices WHERE device_identifier=$1", identifier).Scan(&id)
	if err == nil {
		return id, nil
	}
	return insertDevice(db, identifier)
}

func insertDevice(db *sql.DB, identifier string) (int, error) {
	var id int
	err := db.QueryRow("INSERT INTO devices (device_identifier) VALUES ($1) RETURNING id", identifier).Scan(&id)
	return id, err
}

func getOrCreateSensor(db *sql.DB, name, valueType string) (int, error) {
	var id int
	err := db.QueryRow("SELECT id FROM sensors WHERE name=$1 AND value_type=$2", name, valueType).Scan(&id)
	if err == nil {
		return id, nil
	}
	return insertSensor(db, name, valueType)
}

func insertSensor(db *sql.DB, name, valueType string) (int, error) {
	var id int
	err := db.QueryRow("INSERT INTO sensors (name, value_type) VALUES ($1,$2) RETURNING id", name, valueType).Scan(&id)
	return id, err
}

func save(db *sql.DB, t Telemetry) error {
	_, err := db.Exec(`
		INSERT INTO telemetry_readings (device_id, sensor_type, value_type, value, timestamp)
		VALUES ($1, $2, $3, $4, $5)`,
		t.DeviceID, t.SensorType, t.ValueType, t.Value, t.Timestamp,
	)

	return err
}

func connectRabbitMQ(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	_, err = ch.QueueDeclare("telemetry_queue", true, false, false, false, nil)
	return conn, ch, err
}

func handle(db *sql.DB, msg amqp.Delivery) {
	var t Telemetry

	if err := json.Unmarshal(msg.Body, &t); err != nil {
		log.Println("Invalid message:", err)
		msg.Nack(false, false)
		return
	}

	if err := save(db, t); err != nil {
		log.Println("DB error:", err)
		msg.Nack(false, true)
		return
	}

	log.Printf("Saved: %s %s %.2f", t.DeviceID, t.SensorType, t.Value)
	msg.Ack(false)
}

func consume(ch *amqp.Channel, db *sql.DB, done <-chan struct{}) {
	msgs, err := ch.Consume("telemetry_queue", "", false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case msg := <-msgs:
			handle(db, msg)
		case <-done:
			return
		}
	}
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://admin:admin@rabbitmq:5672/"
	}

	dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        dbURL = "postgres://telemetry:telemetry@postgres:5432/telemetrydb?sslmode=disable"
    }

	var conn *amqp.Connection
	var ch *amqp.Channel
	var db *sql.DB
	var err error

	for i := 0; i < 10; i++ {
		conn, ch, err = connectRabbitMQ(rabbitURL)
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	defer ch.Close()

	for i := 0; i < 10; i++ {
		db, err = connectPostgres(dbURL)
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	done := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		close(done)
	}()

	consume(ch, db, done)
}
