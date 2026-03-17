/**
 * k6 Load Test for LLM Router Platform
 *
 * Targets:
 *   - /v1/chat/completions (core proxy path)
 *   - /api/v1/dashboard/overview
 *   - /health
 *
 * Usage:
 *   k6 run tests/load/k6-load-test.js
 *
 * Environment variables:
 *   BASE_URL   (default http://localhost:8080)
 *   API_KEY    (required — a valid user API key)
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Trend, Counter, Rate } from 'k6/metrics';

// ── Custom Metrics ────────────────────────────────────────────────────
const chatLatency   = new Trend('chat_completions_latency', true);
const dashLatency   = new Trend('dashboard_latency', true);
const healthLatency = new Trend('health_latency', true);
const errorRate     = new Rate('error_rate');
const chatErrors    = new Counter('chat_errors');

// ── Options ───────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    // Smoke: verify the system works under minimal load
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      startTime: '0s',
      tags: { scenario: 'smoke' },
    },
    // Load: normal expected traffic
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 20 },   // ramp up
        { duration: '3m', target: 20 },   // steady state
        { duration: '1m', target: 50 },   // peak
        { duration: '2m', target: 50 },   // hold peak
        { duration: '1m', target: 0 },    // ramp down
      ],
      startTime: '30s',
      tags: { scenario: 'load' },
    },
    // Stress: find breaking point
    stress: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 100 },
        { duration: '2m', target: 100 },
        { duration: '1m', target: 200 },
        { duration: '2m', target: 200 },
        { duration: '1m', target: 0 },
      ],
      startTime: '9m',  // after load scenario
      tags: { scenario: 'stress' },
    },
  },
  thresholds: {
    // P95 latency targets
    'chat_completions_latency': ['p(95)<5000'],  // 5s (upstream LLM dependent)
    'dashboard_latency': ['p(95)<500'],           // 500ms
    'health_latency': ['p(95)<100'],              // 100ms
    // Error rate
    'error_rate': ['rate<0.05'],                  // <5% errors
    // HTTP request duration overall
    'http_req_duration': ['p(95)<5000'],
  },
};

// ── Configuration ─────────────────────────────────────────────────────
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY  = __ENV.API_KEY  || '';

const headers = {
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${API_KEY}`,
};

// ── Test Functions ────────────────────────────────────────────────────

function testHealthEndpoint() {
  const res = http.get(`${BASE_URL}/health`);
  healthLatency.add(res.timings.duration);
  const ok = check(res, {
    'health: status 200': (r) => r.status === 200,
  });
  errorRate.add(!ok);
}

function testDashboardOverview() {
  const res = http.get(`${BASE_URL}/api/v1/dashboard/overview`, { headers });
  dashLatency.add(res.timings.duration);
  const ok = check(res, {
    'dashboard: status 200': (r) => r.status === 200,
    'dashboard: has total_requests': (r) => {
      try { return JSON.parse(r.body).total_requests !== undefined; }
      catch { return false; }
    },
  });
  errorRate.add(!ok);
}

function testChatCompletions() {
  const payload = JSON.stringify({
    model: 'gpt-4o-mini',
    messages: [
      { role: 'user', content: 'Say "load test ok" in exactly 3 words.' },
    ],
    max_tokens: 10,
    temperature: 0,
  });

  const res = http.post(`${BASE_URL}/v1/chat/completions`, payload, { headers });
  chatLatency.add(res.timings.duration);

  const ok = check(res, {
    'chat: status 200': (r) => r.status === 200,
    'chat: has choices': (r) => {
      try { return JSON.parse(r.body).choices?.length > 0; }
      catch { return false; }
    },
  });

  if (!ok) chatErrors.add(1);
  errorRate.add(!ok);
}

// ── Main VU Loop ──────────────────────────────────────────────────────
export default function () {
  group('Health Check', () => {
    testHealthEndpoint();
  });

  group('Dashboard API', () => {
    testDashboardOverview();
  });

  if (API_KEY) {
    group('Chat Completions', () => {
      testChatCompletions();
    });
  }

  sleep(1); // 1 req/sec per VU baseline
}

// ── Summary ───────────────────────────────────────────────────────────
export function handleSummary(data) {
  const summary = {
    timestamp: new Date().toISOString(),
    scenarios: Object.keys(options.scenarios),
    thresholds: {},
    metrics: {},
  };

  // Capture threshold pass/fail
  for (const [name, vals] of Object.entries(data.metrics)) {
    if (vals.thresholds) {
      summary.thresholds[name] = vals.thresholds;
    }
    if (vals.values) {
      summary.metrics[name] = vals.values;
    }
  }

  return {
    'tests/load/results/summary.json': JSON.stringify(summary, null, 2),
    stdout: textSummary(data, { indent: '  ', enableColors: true }),
  };
}

import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.3/index.js';
