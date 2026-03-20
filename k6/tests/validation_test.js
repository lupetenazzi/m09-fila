/**
 * VALIDATION TEST — Regras de negócio
 * 3 VUs / 1min — garante que a API aceita válidos e rejeita inválidos.
 *
 * Cenários: payload analógico, discreto, campo ausente,
 *           discrete value inválido, JSON malformado, body vazio.
 */

import http from "k6/http";
import { check, group, sleep } from "k6";
import { Counter, Rate } from "k6/metrics";
import {
  generateTelemetryPayload,
  generateInvalidPayload,
  generateDiscreteInvalidPayload,
  HEADERS,
  BASE_URL,
} from "../lib/helpers.js";

const validOkCount        = new Counter("validation_valid_ok");
const invalidRejectedCount = new Counter("validation_invalid_rejected");
const unexpectedErrorRate  = new Rate("validation_unexpected_errors");

export const options = {
  scenarios: {
    validation: { executor: "constant-vus", vus: 3, duration: "1m" },
  },
  thresholds: {
    validation_unexpected_errors: ["rate==0"],
  },
};

export default function () {
  group("payload analógico válido", () => {
    const p = { ...generateTelemetryPayload(), value_type: "analog", value: 42.7 };
    const r = http.post(`${BASE_URL}/telemetry`, JSON.stringify(p), { headers: HEADERS });
    const ok = check(r, {
      "analógico → 200":           (r) => r.status === 200,
      "body contém queued":        (r) => r.body.includes("queued"),
    });
    if (ok) validOkCount.add(1);
    if (r.status === 500) unexpectedErrorRate.add(1);
  });

  sleep(0.1);

  group("payload discreto válido", () => {
    const p = { ...generateTelemetryPayload(), value_type: "discrete", value: 0 };
    const r = http.post(`${BASE_URL}/telemetry`, JSON.stringify(p), { headers: HEADERS });
    const ok = check(r, { "discreto 0 → 200": (r) => r.status === 200 });
    if (ok) validOkCount.add(1);
  });

  sleep(0.1);

  group("campo obrigatório ausente", () => {
    const r = http.post(`${BASE_URL}/telemetry`, JSON.stringify(generateInvalidPayload()), { headers: HEADERS });
    const ok = check(r, {
      "campo ausente → 400": (r) => r.status === 400,
      "body com error":      (r) => r.body.includes("error"),
    });
    if (ok) invalidRejectedCount.add(1);
    if (r.status === 500) unexpectedErrorRate.add(1);
  });

  sleep(0.1);

  group("discrete value inválido", () => {
    const r = http.post(`${BASE_URL}/telemetry`, JSON.stringify(generateDiscreteInvalidPayload()), { headers: HEADERS });
    const ok = check(r, {
      "discrete inválido → 400":    (r) => r.status === 400,
      "mensagem menciona discrete":  (r) => r.body.includes("discrete"),
    });
    if (ok) invalidRejectedCount.add(1);
  });

  sleep(0.1);

  group("JSON malformado", () => {
    const r = http.post(`${BASE_URL}/telemetry`, "{invalid{{", { headers: HEADERS });
    check(r, { "JSON inválido → 400": (r) => r.status === 400 });
  });

  sleep(0.1);

  group("body vazio", () => {
    const r = http.post(`${BASE_URL}/telemetry`, "", { headers: HEADERS });
    check(r, { "body vazio → 400": (r) => r.status === 400 });
  });

  sleep(0.3);
}

export function handleSummary(data) {
  return {
    stdout: summary("VALIDATION TEST", data),
  };
}

function summary(data) {
  const m = data.metrics;
  return `
╔══════════════════════════════════════════════════════════╗
║               RESULTADO: VALIDATION TEST                 ║
╠══════════════════════════════════════════════════════════╣
║  Válidos aceitos (200)     : ${String(m.validation_valid_ok?.values?.count ?? 0).padEnd(29)}║
║  Inválidos rejeitados (400): ${String(m.validation_invalid_rejected?.values?.count ?? 0).padEnd(29)}║
║  Erros inesperados (5xx)   : ${String(((m.validation_unexpected_errors?.values?.rate ?? 0) * 100).toFixed(2) + "%").padEnd(29)}║
╠══════════════════════════════════════════════════════════╣
║  5xx inesperado = 0% : ${(m.validation_unexpected_errors?.values?.rate === 0 ? "✅ PASSOU" : "❌ FALHOU").padEnd(35)}║
╚══════════════════════════════════════════════════════════╝
`;
}