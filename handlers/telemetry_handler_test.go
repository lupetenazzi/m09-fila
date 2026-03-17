package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"telemetry-backend/rabbitmq"
)

// -------------------- MOCK --------------------

type MockPublisher struct {
	Messages [][]byte
	Fail     bool
}

func (m *MockPublisher) Publish(message []byte) error {
	if m.Fail {
		return fmt.Errorf("failed to publish")
	}
	m.Messages = append(m.Messages, message)
	return nil
}

// -------------------- SETUP --------------------

func setupRouter(mock rabbitmq.Publisher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	handler := NewTelemetryHandler(mock)
	router.POST("/telemetry", handler.ReceiveTelemetry)

	return router
}

// -------------------- TESTS --------------------

func TestReceiveTelemetry_Success(t *testing.T) {
	mock := &MockPublisher{}
	router := setupRouter(mock)

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

	if len(mock.Messages) != 1 {
		t.Errorf("expected 1 message published, got %d", len(mock.Messages))
	}
}

func TestReceiveTelemetry_InvalidJSON(t *testing.T) {
	mock := &MockPublisher{}
	router := setupRouter(mock)

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

	if len(mock.Messages) != 0 {
		t.Errorf("expected 0 messages published, got %d", len(mock.Messages))
	}
}

func TestReceiveTelemetry_InvalidDiscreteValue(t *testing.T) {
	mock := &MockPublisher{}
	router := setupRouter(mock)

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

	if len(mock.Messages) != 0 {
		t.Errorf("expected 0 messages published, got %d", len(mock.Messages))
	}
}

func TestReceiveTelemetry_MissingField(t *testing.T) {
	mock := &MockPublisher{}
	router := setupRouter(mock)

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

	if len(mock.Messages) != 0 {
		t.Errorf("expected 0 messages published, got %d", len(mock.Messages))
	}
}

func TestReceiveTelemetry_PublishError(t *testing.T) {
	mock := &MockPublisher{Fail: true}
	router := setupRouter(mock)

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

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}