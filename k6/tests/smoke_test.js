/**
 * SMOKE TEST — Validação de baseline
 * 5 VUs / 30s — a API está respondendo corretamente?
 *
 * Thresholds: erro < 1%, p95 < 500ms
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { generateTelemetryPayload, HEADERS, BASE_URL } from "../lib/helpers.js";

const errorRate    = new Rate("custom_error_rate");
const latencyTrend = new Trend("custom_latency_ms", true);
const successCount = new Counter("custom_success_total");

export const options = {
  scenarios: {
    smoke: { executor: "constant-vus", vus: 5, duration: "30s" },
  },
  thresholds: {
    http_req_failed:   ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000", "p(50)<200"],
    custom_error_rate: ["rate<0.01"],
  },
};

export default function () {
  const res = http.post(
    `${BASE_URL}/telemetry`,
    JSON.stringify(generateTelemetryPayload()),
    { headers: HEADERS }
  );

  const ok = check(res, {
    "status é 200":        (r) => r.status === 200,
    "body contém queued":  (r) => r.body.includes("queued"),
    "latência < 1s":       (r) => r.timings.duration < 1000,
  });

  errorRate.add(!ok);
  latencyTrend.add(res.timings.duration);
  if (ok) successCount.add(1);

  sleep(0.5);
}

export function handleSummary(data) {
  return {
    stdout: summary("SMOKE TEST", data),
  };
}

function summary(title, data) {
  const m   = data.metrics;
  const dur = (v) => (v != null ? `${v.toFixed(1)}ms` : "N/A");
  const pct = (v) => (v != null ? `${(v * 100).toFixed(2)}%` : "N/A");

  const d = m.http_req_duration?.values;

  return `
╔══════════════════════════════════════════════════╗
║  ${title.padEnd(46)}║
╠══════════════════════════════════════════════════╣
║  Requisições  : ${String(m.http_reqs?.values?.count ?? "N/A").padEnd(31)}║
║  Throughput   : ${String((m.http_reqs?.values?.rate ?? 0).toFixed(2) + " req/s").padEnd(31)}║
║  Taxa de erro : ${pct(m.http_req_failed?.values?.rate).padEnd(31)}║
║  p50          : ${dur(d?.med).padEnd(31)}║
║  p95          : ${dur(d?.["p(95)"]).padEnd(31)}║
║  p99          : ${dur(d?.["p(99)"]).padEnd(31)}║
║  max          : ${dur(d?.max).padEnd(31)}║
╚══════════════════════════════════════════════════╝
`;
}