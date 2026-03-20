/**
 * SOAK TEST — Resistência prolongada
 * 30 VUs constantes por 10 minutos.
 * Detecta: memory leak, pool de conexões esgotado, degradação gradual.
 *
 * Thresholds: erro < 1%, p95 < 1000ms, p99 < 2000ms
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { generateTelemetryPayload, HEADERS, BASE_URL } from "../lib/helpers.js";

const errorRate    = new Rate("custom_error_rate");
const latencyTrend = new Trend("custom_latency_ms", true);
const successCount = new Counter("custom_success_total");
const dbErrorCount = new Counter("custom_db_error_total");

export const options = {
  scenarios: {
    soak: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "1m", target: 30 },
        { duration: "8m", target: 30 },
        { duration: "1m", target: 0  },
      ],
    },
  },
  thresholds: {
    http_req_failed:   ["rate<0.01"],
    http_req_duration: ["p(95)<1000", "p(99)<2000", "p(50)<500"],
    custom_error_rate: ["rate<0.01"],
  },
};

export default function () {
  const res = http.post(
    `${BASE_URL}/telemetry`,
    JSON.stringify(generateTelemetryPayload()),
    { headers: HEADERS }
  );

  if (res.status === 500 && res.body.includes("connection")) dbErrorCount.add(1);

  const ok = check(res, {
    "status é 200":        (r) => r.status === 200,
    "sem erro de pool":    (r) => r.status !== 503,
    "latência estável":    (r) => r.timings.duration < 2000,
  });

  errorRate.add(!ok);
  latencyTrend.add(res.timings.duration);
  if (ok) successCount.add(1);

  sleep(0.5 + Math.random() * 0.5);
}

export function handleSummary(data) {
  return {
    "/results/soak_summary.json": JSON.stringify(data, null, 2),
    stdout: summary(data),
  };
}

function summary(data) {
  const m        = data.metrics;
  const dur      = (v) => (v ? `${v.toFixed(1)}ms` : "N/A");
  const pct      = (v) => (v != null ? `${(v * 100).toFixed(2)}%` : "N/A");
  const ok       = (pass) => pass ? "✅ PASSOU" : "❌ FALHOU";
  const dbErrors = m.custom_db_error_total?.values?.count ?? 0;
  return `
╔══════════════════════════════════════════════════════════╗
║                    RESULTADO: SOAK TEST                  ║
╠══════════════════════════════════════════════════════════╣
║  Requisições  : ${String(m.http_reqs?.values?.count ?? "N/A").padEnd(41)}║
║  Throughput   : ${String((m.http_reqs?.values?.rate ?? 0).toFixed(2) + " req/s").padEnd(41)}║
║  Erros de pool: ${String(dbErrors).padEnd(41)}║
║  Taxa de erro : ${pct(m.http_req_failed?.values?.rate).padEnd(41)}║
╠══════════════════════════════════════════════════════════╣
║  p50 : ${dur(m.http_req_duration?.values?.["p(50)"]).padEnd(51)}║
║  p90 : ${dur(m.http_req_duration?.values?.["p(90)"]).padEnd(51)}║
║  p95 : ${dur(m.http_req_duration?.values?.["p(95)"]).padEnd(51)}║
║  p99 : ${dur(m.http_req_duration?.values?.["p(99)"]).padEnd(51)}║
║  max : ${dur(m.http_req_duration?.values?.max).padEnd(51)}║
╠══════════════════════════════════════════════════════════╣
║  erro < 1%       : ${ok(m.http_req_failed?.values?.rate < 0.01).padEnd(39)}║
║  p95 < 1000ms    : ${ok(m.http_req_duration?.values?.["p(95)"] < 1000).padEnd(39)}║
║  p99 < 2000ms    : ${ok(m.http_req_duration?.values?.["p(99)"] < 2000).padEnd(39)}║
║  sem erros pool  : ${ok(dbErrors === 0).padEnd(39)}║
╚══════════════════════════════════════════════════════════╝
`;
}