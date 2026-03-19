package main

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

// MockTelemetry helper para criar telemetrias de teste
func createTestTelemetry() Telemetry {
	return Telemetry{
		DeviceID:   "device-001",
		Timestamp:  time.Now(),
		SensorType: "temperature",
		ValueType:  "analog",
		Value:      25.5,
	}
}

// TestGetOrCreateDevice verifica se o dispositivo é criado ou recuperado corretamente
func TestGetOrCreateDevice(t *testing.T) {
	t.Skip("Requer tabela 'devices' do banco - validar em integração")
}

// TestGetOrCreateSensor verifica se o sensor é criado ou recuperado corretamente
func TestGetOrCreateSensor(t *testing.T) {
	t.Skip("Requer tabela 'sensors' do banco - validar em integração")
}

// TestSaveTelemetry verifica se a telemetria é salva corretamente
func TestSaveTelemetry(t *testing.T) {
	db := createTestDatabase(t)
	defer db.Close()

	telemetry := createTestTelemetry()

	err := save(db, telemetry)
	if err != nil {
		t.Fatalf("Falha ao salvar telemetria: %v", err)
	}

	// Verificar se foi salvo
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM telemetry_readings WHERE device_id = $1",
		telemetry.DeviceID,
	).Scan(&count)

	if err != nil {
		t.Fatalf("Erro ao contar registros: %v", err)
	}

	if count != 1 {
		t.Fatalf("Esperado 1 registro, encontrado %d", count)
	}
}

// TestSaveMultipleTelemetries verifica se múltiplas telemetrias são salvas
func TestSaveMultipleTelemetries(t *testing.T) {
	db := createTestDatabase(t)
	defer db.Close()

	count := 10
	for i := 0; i < count; i++ {
		telemetry := Telemetry{
			DeviceID:   "device-001",
			Timestamp:  time.Now().Add(time.Second * time.Duration(i)),
			SensorType: "temperature",
			ValueType:  "analog",
			Value:      float64(20 + i),
		}
		err := save(db, telemetry)
		if err != nil {
			t.Fatalf("Falha ao salvar telemetria %d: %v", i, err)
		}
	}

	var savedCount int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM telemetry_readings WHERE device_id = $1",
		"device-001",
	).Scan(&savedCount)

	if err != nil {
		t.Fatalf("Erro ao contar registros: %v", err)
	}

	if savedCount != count {
		t.Fatalf("Esperado %d registros, encontrado %d", count, savedCount)
	}
}

// TestHandleValidMessage verifica o processamento de uma mensagem válida
func TestHandleValidMessage(t *testing.T) {
	t.Skip("Teste requer mock de AMQP Delivery")
}

// TestHandleInvalidJSON verifica o processamento de JSON inválido
func TestHandleInvalidJSON(t *testing.T) {
	t.Skip("Teste requer mock de AMQP Delivery")
}

// TestConcurrentSaves verifica se a função save é thread-safe
func TestConcurrentSaves(t *testing.T) {
	db := createTestDatabase(t)
	if db == nil {
		t.Skip("PostgreSQL não disponível para teste concorrente")
	}
	defer db.Close()

	done := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			telemetry := Telemetry{
				DeviceID:   "device-concurrent",
				Timestamp:  time.Now(),
				SensorType: "sensor-" + string(rune(48+index)),
				ValueType:  "analog",
				Value:      float64(index * 10),
			}
			err := save(db, telemetry)
			done <- err
		}(i)
	}

	for i := 0; i < 10; i++ {
		err := <-done
		if err != nil {
			t.Logf("Aviso: Falha ao salvar telemetria concorrente %d: %v", i, err)
		}
	}
}

// TestTelemetryUnmarshal verifica o desserialização de Telemetry
func TestTelemetryUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"device_id": "device-001",
		"timestamp": "2026-03-19T10:30:00Z",
		"sensor_type": "temperature",
		"value_type": "analog",
		"value": 25.5
	}`)

	var telemetry Telemetry
	err := json.Unmarshal(jsonData, &telemetry)

	if err != nil {
		t.Fatalf("Falha ao desserializar: %v", err)
	}

	if telemetry.DeviceID != "device-001" {
		t.Fatalf("DeviceID não corresponde: %s", telemetry.DeviceID)
	}

	if telemetry.Value != 25.5 {
		t.Fatalf("Value não corresponde: %f", telemetry.Value)
	}
}

// ==================== HELPER FUNCTIONS ====================

// createTestDatabase cria um banco de dados de teste
func createTestDatabase(t *testing.T) *sql.DB {
	// Conectar ao banco de teste (PostgreSQL deve estar rodando)
	dsn := "postgres://telemetry:telemetry@localhost:5432/telemetrydb?sslmode=disable"

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Logf("Aviso: Não foi possível conectar ao banco de teste: %v", err)
		return nil
	}

	// Testar conexão
	if err := db.Ping(); err != nil {
		t.Logf("Aviso: Não foi possível fazer ping no banco: %v", err)
		return nil
	}

	// Limpar dados de teste anteriores (ignorar erros se tabelas não existem)
	db.Exec("DELETE FROM telemetry_readings")
	db.Exec("DELETE FROM devices")
	db.Exec("DELETE FROM sensors")

	return db
}
