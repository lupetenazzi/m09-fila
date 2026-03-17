package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// helper para criar router de teste
func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/telemetry", ReceiveTelemetry)
	return router
}

func TestReceiveTelemetry_Success(t *testing.T) {
	router := setupRouter()

	body := `{
		"device_id": "device-123",
		"timestamp": "2026-03-17T10:00:00Z",
		"sensor_type": "temperature",
		"value_type": "analog",
		"value": 23.5
	}`

	req, _ := http.NewRequest(http.MethodPost, "/telemetry", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestReceiveTelemetry_InvalidJSON(t *testing.T) {
	router := setupRouter()

	body := `{
		"device_id": "device-123",
		"timestamp": "invalid-date",
		"sensor_type": "temperature",
		"value_type": "analog",
		"value": 23.5
	}`

	req, _ := http.NewRequest(http.MethodPost, "/telemetry", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestReceiveTelemetry_InvalidDiscreteValue(t *testing.T) {
	router := setupRouter()

	body := `{
		"device_id": "device-123",
		"timestamp": "2026-03-17T10:00:00Z",
		"sensor_type": "switch",
		"value_type": "discrete",
		"value": 5
	}`

	req, _ := http.NewRequest(http.MethodPost, "/telemetry", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestReceiveTelemetry_MissingField(t *testing.T) {
	router := setupRouter()

	body := `{
		"timestamp": "2026-03-17T10:00:00Z",
		"sensor_type": "temperature",
		"value_type": "analog",
		"value": 23.5
	}`

	req, _ := http.NewRequest(http.MethodPost, "/telemetry", bytes.NewBuffer([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}