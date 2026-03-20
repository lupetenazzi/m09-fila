/**
 * LOAD TEST — Carga sustentada
 * Ramp-up 0→50 VUs (1min) → sustentado 50 VUs (3min) → ramp-down (30s)
 *
 * Thresholds: erro < 2%, p95 < 800ms, p99 < 1500ms
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { generateTelemetryPayload, HEADERS, BASE_URL } from "../lib/helpers.js";

const errorRate    = new Rate("custom_error_rate");
const latencyTrend = new Trend("custom_latency_ms", true);
const successCount = new Counter("custom_success_total");
const failCount    = new Counter("custom_fail_total");

export const options = {
  scenarios: {
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "1m",  target: 50 },
        { duration: "3m",  target: 50 },
        { duration: "30s", target: 0  },
      ],
    },
  },
  thresholds: {
    http_req_failed:   ["rate<0.02"],
    http_req_duration: ["p(95)<800", "p(99)<1500", "p(50)<400"],  // adicione p(50) e p(99)
    custom_error_rate: ["rate<0.02"],
  },
};

export default function () {
  const res = http.post(
    `${BASE_URL}/telemetry`,
    JSON.stringify(generateTelemetryPayload()),
    { headers: HEADERS }
  );

  const ok = check(res, {
    "status é 200":       (r) => r.status === 200,
    "mensagem de sucesso": (r) => r.body.includes("queued"),
    "latência < 2s":      (r) => r.timings.duration < 2000,
  });

  errorRate.add(!ok);
  latencyTrend.add(res.timings.duration);
  ok ? successCount.add(1) : failCount.add(1);

  sleep(Math.random() * 0.5 + 0.1);
}

export function handleSummary(data) {
  return {
    stdout: summary("LOAD TEST", data),
  };
}

function summary(data) {
  const m   = data.metrics;
  const dur = (v) => (v ? `${v.toFixed(1)}ms` : "N/A");
  const pct = (v) => (v != null ? `${(v * 100).toFixed(2)}%` : "N/A");
  const ok  = (pass) => pass ? "✅ PASSOU" : "❌ FALHOU";
  return `
╔══════════════════════════════════════════════════════════╗
║                    RESULTADO: LOAD TEST                  ║
╠══════════════════════════════════════════════════════════╣
║  Requisições  : ${String(m.http_reqs?.values?.count ?? "N/A").padEnd(41)}║
║  Throughput   : ${String((m.http_reqs?.values?.rate ?? 0).toFixed(2) + " req/s").padEnd(41)}║
║  Sucessos     : ${String(m.custom_success_total?.values?.count ?? "N/A").padEnd(41)}║
║  Falhas       : ${String(m.custom_fail_total?.values?.count ?? "N/A").padEnd(41)}║
║  Taxa de erro : ${pct(m.http_req_failed?.values?.rate).padEnd(41)}║
╠══════════════════════════════════════════════════════════╣
║  p50 : ${dur(m.http_req_duration?.values?.["p(50)"]).padEnd(51)}║
║  p90 : ${dur(m.http_req_duration?.values?.["p(90)"]).padEnd(51)}║
║  p95 : ${dur(m.http_req_duration?.values?.["p(95)"]).padEnd(51)}║
║  p99 : ${dur(m.http_req_duration?.values?.["p(99)"]).padEnd(51)}║
║  max : ${dur(m.http_req_duration?.values?.max).padEnd(51)}║
╠══════════════════════════════════════════════════════════╣
║  erro < 2%    : ${ok(m.http_req_failed?.values?.rate < 0.02).padEnd(41)}║
║  p95 < 800ms  : ${ok(m.http_req_duration?.values?.["p(95)"] < 800).padEnd(41)}║
║  p99 < 1500ms : ${ok(m.http_req_duration?.values?.["p(99)"] < 1500).padEnd(41)}║
╚══════════════════════════════════════════════════════════╝
`;
}