package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"telemetry-backend/models"
	"telemetry-backend/rabbitmq"
)

type TelemetryHandler struct {
	Rabbit *rabbitmq.RabbitMQClient
}

func NewTelemetryHandler(rabbit *rabbitmq.RabbitMQClient) *TelemetryHandler {
	return &TelemetryHandler{Rabbit: rabbit}
}

func (h *TelemetryHandler) ReceiveTelemetry(c *gin.Context) {
	var telemetry models.Telemetry

	if err := c.ShouldBindJSON(&telemetry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validação adicional
	if telemetry.ValueType == models.Discrete {
		if telemetry.Value != 0 && telemetry.Value != 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "discrete value must be 0 or 1",
			})
			return
		}
	}

	// Serializa para JSON
	body, err := json.Marshal(telemetry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize"})
		return
	}

	// Publica na fila
	err = h.Rabbit.Publish(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "telemetry queued successfully",
	})
}