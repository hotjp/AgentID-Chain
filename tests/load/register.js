// =============================================================================
// AgentID-Chain — k6 Load Test: Register 端点 (P18.5)
// =============================================================================
// 目标：
//   - 100 RPS 持续 5 分钟
//   - P99 延迟 < 100ms
//   - 错误率 < 0.1%
//
// 用法：
//   k6 run tests/load/register.js
//   k6 run --out json=results.json tests/load/register.js
//   BASE_URL=http://localhost:8080 k6 run tests/load/register.js
// =============================================================================

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import crypto from 'k6/crypto';

// 自定义指标
const registerSuccess = new Counter('register_success_total');
const registerFailure = new Counter('register_failure_total');
const errorRate = new Rate('error_rate');
const registerLatency = new Trend('register_latency_ms');

// 配置
export const options = {
  scenarios: {
    register_load: {
      executor: 'constant-arrival-rate',
      rate: 100,                     // 100 RPS
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 20,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(99)<100'],  // P99 < 100ms
    http_req_failed:   ['rate<0.001'], // 错误率 < 0.1%
    error_rate:        ['rate<0.001'],
  },
  noConnectionReuse: false,
  userAgent: 'k6-loadtest/1.0 (AgentID-Chain)',
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const ENDPOINT = '/v1/agents/register';

// 工具函数
function generateUUID() {
  // 简化的 v4 UUID
  const bytes = crypto.randomBytes(16);
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant
  const hex = bytes.map(b => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function generateEd25519PubKey() {
  // 模拟公钥（实际应使用真实 ed25519 密钥对）
  const bytes = crypto.randomBytes(32);
  return bytes.toString('base64');
}

function buildRegisterPayload() {
  return JSON.stringify({
    uuid:         generateUUID(),
    owner:        `loadtest-${randomString(8)}`,
    level:        1,
    public_key:   generateEd25519PubKey(),
    metadata: {
      source: 'k6-loadtest',
      tag:    randomString(6),
    },
  });
}

function buildHeaders() {
  return {
    'Content-Type': 'application/json',
    'X-API-Key':    __ENV.API_KEY || 'dev-loadtest-key',
    'X-Request-ID': `loadtest-${Date.now()}-${randomString(6)}`,
  };
}

// 主测试函数
export default function () {
  const url = `${BASE_URL}${ENDPOINT}`;
  const payload = buildRegisterPayload();
  const headers = buildHeaders();

  const start = Date.now();
  const res = http.post(url, payload, { headers, tags: { name: 'register' } });
  const duration = Date.now() - start;

  registerLatency.add(duration);

  const success = check(res, {
    'status is 201':         r => r.status === 201,
    'response has uuid':     r => {
      try { return r.json('uuid') !== undefined; } catch (e) { return false; }
    },
    'response has tx_hash':  r => {
      try { return r.json('tx_hash') !== undefined; } catch (e) { return false; }
    },
    'latency < 100ms':       r => duration < 100,
  });

  if (success) {
    registerSuccess.add(1);
    errorRate.add(false);
  } else {
    registerFailure.add(1);
    errorRate.add(true);
  }

  // 短暂 pause（避免过载后端）
  sleep(0.01);
}

// 启动前：健康检查
export function setup() {
  console.log(`==> Load test target: ${BASE_URL}${ENDPOINT}`);
  const healthRes = http.get(`${BASE_URL}/live`);
  if (healthRes.status !== 200) {
    console.warn(`Health check returned ${healthRes.status}; continuing anyway`);
  }
  return { startTime: Date.now() };
}

// 结束后：汇总
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`==> Load test completed in ${duration.toFixed(1)}s`);
}

// 错误处理
export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: '  ', enableColors: true }),
    'tests/load/results/register-summary.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data, opts) {
  // 简化版 summary（k6 完整版可用 jslib）
  const m = data.metrics;
  const lines = [
    '',
    '========== Load Test Summary ==========',
    `Total HTTP requests:    ${m.http_reqs?.values?.count || 0}`,
    `HTTP failed:            ${((m.http_req_failed?.values?.rate || 0) * 100).toFixed(3)}%`,
    `HTTP duration P50:      ${(m.http_req_duration?.values?.['p(50)'] || 0).toFixed(2)}ms`,
    `HTTP duration P95:      ${(m.http_req_duration?.values?.['p(95)'] || 0).toFixed(2)}ms`,
    `HTTP duration P99:      ${(m.http_req_duration?.values?.['p(99)'] || 0).toFixed(2)}ms`,
    `Register success:       ${m.register_success_total?.values?.count || 0}`,
    `Register failure:       ${m.register_failure_total?.values?.count || 0}`,
    `Error rate:             ${((m.error_rate?.values?.rate || 0) * 100).toFixed(3)}%`,
    '========================================',
    '',
  ];
  return lines.join('\n');
}
