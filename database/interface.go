package database

import "telemetry-backend/models"

// TelemetryRepository define o contrato para persistência de telemetria.
// Usar interface facilita mock nos testes unitários.
type TelemetryRepository interface {
	Save(t models.Telemetry) error
}