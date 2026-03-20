/**
 * STRESS TEST — Ponto de ruptura
 * Escalonamento progressivo: 0 → 30 → 100 → 200 → 0 VUs
 *
 * Thresholds: erro < 10%, p95 < 3000ms
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { generateTelemetryPayload, HEADERS, BASE_URL } from "../lib/helpers.js";

const errorRate    = new Rate("custom_error_rate");
const latencyTrend = new Trend("custom_latency_ms", true);
const successCount = new Counter("custom_success_total");
const failCount    = new Counter("custom_fail_total");
const timeoutCount = new Counter("custom_timeout_total");

export const options = {
  scenarios: {
    stress: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 30  },
        { duration: "1m",  target: 30  },
        { duration: "30s", target: 100 },
        { duration: "2m",  target: 100 },
        { duration: "30s", target: 200 },
        { duration: "1m",  target: 200 },
        { duration: "1m",  target: 0   },
      ],
      gracefulRampDown: "30s",
    },
  },
  thresholds: {
    http_req_failed:   ["rate<0.10"],
    http_req_duration: ["p(95)<3000", "p(99)<5000", "p(50)<1000", "p(75)<2000"],
    custom_error_rate: ["rate<0.10"],
  },
};

export default function () {
  const res = http.post(
    `${BASE_URL}/telemetry`,
    JSON.stringify(generateTelemetryPayload()),
    { headers: HEADERS, timeout: "5s" }
  );

  const timedOut = res.timings.duration >= 5000;
  if (timedOut) timeoutCount.add(1);

  const ok = check(res, {
    "status é 200":   (r) => r.status === 200,
    "sem timeout":    (r) => r.timings.duration < 5000,
    "resposta válida":(r) => r.status >= 200 && r.status < 500,
  });

  errorRate.add(res.status >= 500 || timedOut);
  latencyTrend.add(res.timings.duration);
  res.status === 200 ? successCount.add(1) : failCount.add(1);

  sleep(Math.random() * 0.2);
}

export function handleSummary(data) {
  return {
    "/results/stress_summary.json": JSON.stringify(data, null, 2),
    stdout: summary(data),
  };
}

function summary(data) {
  const m   = data.metrics;
  const dur = (v) => (v ? `${v.toFixed(1)}ms` : "N/A");
  const pct = (v) => (v != null ? `${(v * 100).toFixed(2)}%` : "N/A");
  const ok  = (pass) => pass ? "✅ PASSOU" : "❌ FALHOU";
  return `
╔══════════════════════════════════════════════════════════╗
║                   RESULTADO: STRESS TEST                 ║
╠══════════════════════════════════════════════════════════╣
║  Requisições  : ${String(m.http_reqs?.values?.count ?? "N/A").padEnd(41)}║
║  Throughput   : ${String((m.http_reqs?.values?.rate ?? 0).toFixed(2) + " req/s").padEnd(41)}║
║  Sucessos     : ${String(m.custom_success_total?.values?.count ?? "N/A").padEnd(41)}║
║  Falhas       : ${String(m.custom_fail_total?.values?.count ?? "N/A").padEnd(41)}║
║  Timeouts     : ${String(m.custom_timeout_total?.values?.count ?? "N/A").padEnd(41)}║
║  Taxa de erro : ${pct(m.http_req_failed?.values?.rate).padEnd(41)}║
╠══════════════════════════════════════════════════════════╣
║  p50 : ${dur(m.http_req_duration?.values?.["p(50)"]).padEnd(51)}║
║  p75 : ${dur(m.http_req_duration?.values?.["p(75)"]).padEnd(51)}║
║  p90 : ${dur(m.http_req_duration?.values?.["p(90)"]).padEnd(51)}║
║  p95 : ${dur(m.http_req_duration?.values?.["p(95)"]).padEnd(51)}║
║  p99 : ${dur(m.http_req_duration?.values?.["p(99)"]).padEnd(51)}║
║  max : ${dur(m.http_req_duration?.values?.max).padEnd(51)}║
╠══════════════════════════════════════════════════════════╣
║  erro < 10%   : ${ok(m.http_req_failed?.values?.rate < 0.10).padEnd(41)}║
║  p95 < 3000ms : ${ok(m.http_req_duration?.values?.["p(95)"] < 3000).padEnd(41)}║
╚══════════════════════════════════════════════════════════╝
`;
}