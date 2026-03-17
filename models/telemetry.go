package models

import "time"

type ValueType string

const (
	Analog   ValueType = "analog"
	Discrete ValueType = "discrete"
)

type Telemetry struct {
	DeviceID   string    `json:"device_id" binding:"required"`
	Timestamp  time.Time `json:"timestamp" binding:"required"`
	SensorType string    `json:"sensor_type" binding:"required"`
	ValueType  ValueType `json:"value_type" binding:"required,oneof=analog discrete"`
	Value      float64   `json:"value" binding:"required"`
}