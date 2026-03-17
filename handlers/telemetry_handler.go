package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"telemetry-backend/models"
)

func ReceiveTelemetry(c *gin.Context) {
	var telemetry models.Telemetry

	// Bind e valida JSON
	if err := c.ShouldBindJSON(&telemetry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
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

	// Processamento (simples)
	processTelemetry(telemetry)

	c.JSON(http.StatusOK, gin.H{
		"message": "telemetry received successfully",
	})
}

func processTelemetry(t models.Telemetry) {
	// Aqui entraria persistência, fila, etc.
	println("Device:", t.DeviceID)
	println("Sensor:", t.SensorType)
	println("Value:", t.Value)
}