package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
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

	// 🔁 Retry de conexão
	for i := 0; i < 10; i++ {
		rabbitClient, err = rabbitmq.NewRabbitMQClient(rabbitURL, "telemetry_queue")
		if err == nil {
			log.Println("Connected to RabbitMQ")
			break
		}

		log.Println("RabbitMQ not ready, retrying in 3s...")
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
	}

	router := gin.Default()

	handler := handlers.NewTelemetryHandler(rabbitClient)
	router.POST("/telemetry", handler.ReceiveTelemetry)

	log.Println("Server running on port 8080")
	router.Run(":8080")
}