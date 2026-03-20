/**
 * SPIKE TEST — Burst súbito de tráfego
 * 10 VUs normais → pico de 300 VUs em 5s → recovery
 * Foco: capacidade de enfileiramento do RabbitMQ sob burst.
 *
 * Thresholds: erro < 15%, p99 < 5000ms
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { generateTelemetryPayload, HEADERS, BASE_URL } from "../lib/helpers.js";

const errorRate    = new Rate("custom_error_rate");
const latencyTrend = new Trend("custom_latency_ms", true);
const successCount = new Counter("custom_success_total");
const failCount    = new Counter("custom_fail_total");
const queuedCount  = new Counter("custom_queued_total");

export const options = {
  scenarios: {
    spike: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 10  },
        { duration: "30s", target: 10  },
        { duration: "5s",  target: 300 },
        { duration: "1m",  target: 300 },
        { duration: "5s",  target: 10  },
        { duration: "1m",  target: 10  },
        { duration: "10s", target: 0   },
      ],
      gracefulRampDown: "10s",
    },
  },
  thresholds: {
    http_req_failed:   ["rate<0.15"],
    http_req_duration: ["p(99)<5000", "p(50)<2000", "p(95)<3000"],
    custom_error_rate: ["rate<0.15"],
  },
};

export default function () {
  const res = http.post(
    `${BASE_URL}/telemetry`,
    JSON.stringify(generateTelemetryPayload()),
    { headers: HEADERS, timeout: "8s" }
  );

  if (res.status === 200 && res.body.includes("queued")) queuedCount.add(1);

  const ok = check(res, {
    "status é 200": (r) => r.status === 200,
    "não é 5xx":    (r) => r.status < 500,
    "sem timeout":  (r) => r.timings.duration < 8000,
  });

  errorRate.add(res.status >= 500 || res.timings.duration >= 8000);
  latencyTrend.add(res.timings.duration);
  ok ? successCount.add(1) : failCount.add(1);

  // Sem sleep no pico para maximizar pressão
  sleep(__VU <= 10 ? 0.3 : 0);
}

export function handleSummary(data) {
  return {
    stdout: summary("SPIKE TEST", data),
  };
}

function summary(data) {
  const m   = data.metrics;
  const dur = (v) => (v ? `${v.toFixed(1)}ms` : "N/A");
  const pct = (v) => (v != null ? `${(v * 100).toFixed(2)}%` : "N/A");
  const ok  = (pass) => pass ? "✅ PASSOU" : "❌ FALHOU";
  const total   = m.http_reqs?.values?.count ?? 0;
  const queued  = m.custom_queued_total?.values?.count ?? 0;
  const qRate   = total > 0 ? ((queued / total) * 100).toFixed(1) : "0";
  return `
╔══════════════════════════════════════════════════════════╗
║                    RESULTADO: SPIKE TEST                 ║
╠══════════════════════════════════════════════════════════╣
║  ENFILEIRAMENTO                                          ║
║  Total enviado          : ${String(total).padEnd(31)}║
║  Enfileirado com êxito  : ${String(queued).padEnd(31)}║
║  Taxa de enfileiramento : ${String(qRate + "%").padEnd(31)}║
╠══════════════════════════════════════════════════════════╣
║  p50 : ${dur(m.http_req_duration?.values?.["p(50)"]).padEnd(51)}║
║  p95 : ${dur(m.http_req_duration?.values?.["p(95)"]).padEnd(51)}║
║  p99 : ${dur(m.http_req_duration?.values?.["p(99)"]).padEnd(51)}║
║  max : ${dur(m.http_req_duration?.values?.max).padEnd(51)}║
╠══════════════════════════════════════════════════════════╣
║  Taxa de erro : ${pct(m.http_req_failed?.values?.rate).padEnd(41)}║
╠══════════════════════════════════════════════════════════╣
║  erro < 15%   : ${ok(m.http_req_failed?.values?.rate < 0.15).padEnd(41)}║
║  p99 < 5000ms : ${ok(m.http_req_duration?.values?.["p(99)"] < 5000).padEnd(41)}║
╚══════════════════════════════════════════════════════════╝
`;
}