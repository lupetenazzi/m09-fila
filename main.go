package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"telemetry-backend/handlers"
)

func main() {
	router := gin.Default()

	// Rotas
	router.POST("/telemetry", handlers.ReceiveTelemetry)

	log.Println("Server running on port 8080")
	router.Run(":8080")
}