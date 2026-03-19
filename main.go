package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"telemetry-backend/database"
	"telemetry-backend/handlers"
	"telemetry-backend/rabbitmq"
)

func main() {

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://admin:admin@rabbitmq:5672/"
	}

	var rabbitClient *rabbitmq.RabbitMQClient
	var err error

	for i := 0; i < 10; i++ {
		rabbitClient, err = rabbitmq.NewRabbitMQClient(rabbitURL, "telemetry_queue")
		if err == nil {
			break
		}
		log.Println("RabbitMQ not ready, retrying in 3s...")
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://telemetry:telemetry@postgres:5432/telemetrydb?sslmode=disable"
	}

	var pgClient *database.PostgresClient

	for i := 0; i < 10; i++ {
		pgClient, err = database.NewPostgresClient(dbURL)
		if err == nil {
			break
		}
		log.Println("PostgreSQL not ready, retrying in 3s...")
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL:", err)
	}
	defer pgClient.Close()

	router := gin.Default()

	handler := handlers.NewTelemetryHandler(rabbitClient)
	router.POST("/telemetry", handler.ReceiveTelemetry)

	log.Println("Server running on port 8080")
	router.Run(":8080")
}