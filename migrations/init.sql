
CREATE TYPE value_type AS ENUM ('analog', 'discrete');

-- Tabela principal de leituras de telemetria
CREATE TABLE IF NOT EXISTS telemetry_readings (
    id            BIGSERIAL PRIMARY KEY,
    device_id     VARCHAR(255)             NOT NULL,
    sensor_type   VARCHAR(100)             NOT NULL,
    value_type    value_type               NOT NULL,
    value         DOUBLE PRECISION         NOT NULL,
    timestamp     TIMESTAMPTZ              NOT NULL,
    received_at   TIMESTAMPTZ              NOT NULL DEFAULT NOW()
);

-- Índices para consultas analíticas frequentes
CREATE INDEX idx_telemetry_device_id      ON telemetry_readings (device_id);
CREATE INDEX idx_telemetry_sensor_type    ON telemetry_readings (sensor_type);
CREATE INDEX idx_telemetry_timestamp      ON telemetry_readings (timestamp DESC);
CREATE INDEX idx_telemetry_device_time    ON telemetry_readings (device_id, timestamp DESC);