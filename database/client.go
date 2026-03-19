package database

import (
	"database/sql"
	"fmt"
	"log"

	"telemetry-backend/models"

	_ "github.com/lib/pq"
)

// PostgresClient implementa TelemetryRepository usando PostgreSQL.
type PostgresClient struct {
	db *sql.DB
}

// NewPostgresClient abre a conexão e verifica que o banco está acessível.
func NewPostgresClient(dsn string) (*PostgresClient, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	log.Println("Connected to PostgreSQL")
	return &PostgresClient{db: db}, nil
}

// Save persiste uma leitura de telemetria na tabela telemetry_readings.
func (p *PostgresClient) Save(t models.Telemetry) error {
	const query = `
		INSERT INTO telemetry_readings
			(device_id, sensor_type, value_type, value, timestamp)
		VALUES
			($1, $2, $3, $4, $5)
	`
	_, err := p.db.Exec(query,
		t.DeviceID,
		t.SensorType,
		string(t.ValueType),
		t.Value,
		t.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("database: save telemetry: %w", err)
	}
	return nil
}

// Close encerra a conexão com o banco.
func (p *PostgresClient) Close() error {
	return p.db.Close()
}