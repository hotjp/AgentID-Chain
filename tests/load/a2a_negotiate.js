// =============================================================================
// AgentID-Chain — k6 Load Test: A2A Negotiate 端点 (P18.6)
// =============================================================================
// 目标：
//   - 200 RPS 持续负载
//   - P99 延迟 < 30ms（协商端点应快，无业务计算）
//   - 错误率 < 0.1%
//
// A2A negotiate 流程：
//   1. 客户端 POST /v1/a2a/negotiate 带公钥
//   2. 服务器返回协商结果（capabilities、agent_id 派生、token）
//
// 用法：
//   k6 run tests/load/a2a_negotiate.js
// =============================================================================

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import crypto from 'k6/crypto';

const negotiateSuccess = new Counter('a2a_negotiate_success_total');
const negotiateFailure = new Counter('a2a_negotiate_failure_total');
const errorRate = new Rate('a2a_error_rate');
const negotiateLatency = new Trend('a2a_negotiate_latency_ms');

export const options = {
  scenarios: {
    a2a_negotiate_load: {
      executor: 'constant-arrival-rate',
      rate: 200,                    // 200 RPS
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 30,
      maxVUs: 300,
    },
  },
  thresholds: {
    http_req_duration: ['p(99)<30'],   // P99 < 30ms（协商快路径）
    http_req_failed:   ['rate<0.001'],
    a2a_error_rate:    ['rate<0.001'],
  },
  noConnectionReuse: false,
  userAgent: 'k6-a2a-loadtest/1.0 (AgentID-Chain)',
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const ENDPOINT = '/v1/a2a/negotiate';

function generateEd25519PubKey() {
  return crypto.randomBytes(32).toString('base64');
}

function buildNegotiatePayload() {
  return JSON.stringify({
    public_key:  generateEd25519PubKey(),
    protocol:    'a2a-v1',
    target_aap:  'agentid-chain-v2',
    capabilities: ['identity', 'signature', 'revocation'],
    nonce:       crypto.randomBytes(16).toString('hex'),
  });
}

function buildHeaders() {
  return {
    'Content-Type': 'application/json',
    'X-API-Key':    __ENV.API_KEY || 'dev-loadtest-key',
    'X-Request-ID': `a2a-${Date.now()}-${randomString(6)}`,
  };
}

export default function () {
  const url = `${BASE_URL}${ENDPOINT}`;
  const payload = buildNegotiatePayload();
  const headers = buildHeaders();

  const start = Date.now();
  const res = http.post(url, payload, { headers, tags: { name: 'a2a_negotiate' } });
  const duration = Date.now() - start;

  negotiateLatency.add(duration);

  const success = check(res, {
    'status is 200':           r => r.status === 200,
    'response has session_id': r => {
      try { return r.json('session_id') !== undefined || r.json('token') !== undefined; } catch (e) { return false; }
    },
    'response has expires_at': r => {
      try { return r.json('expires_at') !== undefined; } catch (e) { return false; }
    },
    'latency < 30ms':          r => duration < 30,
  });

  if (success) {
    negotiateSuccess.add(1);
    errorRate.add(false);
  } else {
    negotiateFailure.add(1);
    errorRate.add(true);
  }

  sleep(0.005);
}

export function setup() {
  console.log(`==> A2A negotiate load test: ${BASE_URL}${ENDPOINT}`);
  const healthRes = http.get(`${BASE_URL}/live`);
  if (healthRes.status !== 200) {
    console.warn(`Health check returned ${healthRes.status}; continuing`);
  }
  return { startTime: Date.now() };
}

export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`==> A2A load test completed in ${duration.toFixed(1)}s`);
}

export function handleSummary(data) {
  return {
    'stdout': a2aTextSummary(data),
    'tests/load/results/a2a-negotiate-summary.json': JSON.stringify(data, null, 2),
  };
}

function a2aTextSummary(data) {
  const m = data.metrics;
  return [
    '',
    '========== A2A Negotiate Load Summary ==========',
    `Total requests:    ${m.http_reqs?.values?.count || 0}`,
    `HTTP failed:       ${((m.http_req_failed?.values?.rate || 0) * 100).toFixed(3)}%`,
    `P50 latency:       ${(m.http_req_duration?.values?.['p(50)'] || 0).toFixed(2)}ms`,
    `P95 latency:       ${(m.http_req_duration?.values?.['p(95)'] || 0).toFixed(2)}ms`,
    `P99 latency:       ${(m.http_req_duration?.values?.['p(99)'] || 0).toFixed(2)}ms`,
    `Negotiate success: ${m.a2a_negotiate_success_total?.values?.count || 0}`,
    `Negotiate failure: ${m.a2a_negotiate_failure_total?.values?.count || 0}`,
    `Error rate:        ${((m.a2a_error_rate?.values?.rate || 0) * 100).toFixed(3)}%`,
    '=================================================',
    '',
  ].join('\n');
}
