export const BASE_URL = __ENV.BASE_URL || "http://backend:8080";

export const HEADERS = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

const DEVICE_IDS = Array.from({ length: 50 }, (_, i) =>
  `device-${String(i + 1).padStart(4, "0")}`
);

const SENSOR_TYPES = [
  "temperature", "humidity", "pressure", "vibration",
  "voltage", "current", "rpm", "flow_rate", "co2_level", "light_intensity",
];

const ANALOG_RANGES = {
  temperature:     [-20, 150],
  humidity:        [0, 100],
  pressure:        [900, 1100],
  vibration:       [0, 50],
  voltage:         [0, 250],
  current:         [0, 100],
  rpm:             [0, 10000],
  flow_rate:       [0, 500],
  co2_level:       [300, 5000],
  light_intensity: [0, 100000],
};

export function randomFrom(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export function randomFloat(min, max) {
  // garante que nunca retorna exatamente 0
  const val = Math.random() * (max - min) + min;
  return parseFloat((val === 0 ? 0.0001 : val).toFixed(4));
}

export function generateTelemetryPayload() {
  const sensorType = randomFrom(SENSOR_TYPES);
  const isDiscrete = Math.random() < 0.2;
  const valueType  = isDiscrete ? "discrete" : "analog";
  // discrete: usa 1 apenas — nunca 0, pois binding:"required" rejeita zero
  const value      = isDiscrete
    ? 1
    : randomFloat(...(ANALOG_RANGES[sensorType] || [0.0001, 100]));

  return {
    device_id:   randomFrom(DEVICE_IDS),
    timestamp:   new Date().toISOString(),
    sensor_type: sensorType,
    value_type:  valueType,
    value,
  };
}

export function generateInvalidPayload() {
  const fields = ["device_id", "timestamp", "sensor_type", "value_type", "value"];
  const base   = generateTelemetryPayload();
  delete base[randomFrom(fields)];
  return base;
}

export function generateDiscreteInvalidPayload() {
  return {
    device_id:   randomFrom(DEVICE_IDS),
    timestamp:   new Date().toISOString(),
    sensor_type: "switch",
    value_type:  "discrete",
    value:       5,
  };
}