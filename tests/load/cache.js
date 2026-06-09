// =============================================================================
// AgentID-Chain — k6 Load Test: 缓存命中 (P18.7)
// =============================================================================
// 目标：
//   - 1000 RPS（高读负载）
//   - P99 延迟 < 5ms（缓存命中）
//   - 错误率 < 0.1%
//
// 测试场景：连续 GET 已注册 agent（应走 Redis 缓存路径）
// =============================================================================

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import crypto from 'k6/crypto';

const cacheHit = new Counter('cache_hit_total');
const cacheMiss = new Counter('cache_miss_total');
const errorRate = new Rate('cache_error_rate');
const lookupLatency = new Trend('cache_lookup_latency_ms');

export const options = {
  scenarios: {
    cache_read: {
      executor: 'constant-arrival-rate',
      rate: 1000,                   // 1000 RPS
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 50,
      maxVUs: 500,
    },
  },
  thresholds: {
    http_req_duration: ['p(99)<5'],    // P99 < 5ms（缓存命中目标）
    http_req_failed:   ['rate<0.001'],
    cache_error_rate:  ['rate<0.001'],
  },
  noConnectionReuse: false,
  userAgent: 'k6-cache-loadtest/1.0 (AgentID-Chain)',
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// 预定义一组"热门" agent UUID（应被缓存）
const HOT_AGENTS = Array.from({ length: 100 }, (_, i) =>
  `00000000-0000-4000-8000-${String(i).padStart(12, '0')}`
);

let nextIndex = 0;

function nextAgent() {
  // 90% 命中热点（缓存），10% 随机（模拟穿透）
  const uuid = (Math.random() < 0.9)
    ? HOT_AGENTS[nextIndex++ % HOT_AGENTS.length]
    : crypto.randomBytes(16).toString('hex').replace(/(.{8})(.{4})(.{4})(.{4})(.{12})/, '$1-$2-$3-$4-$5');
  return uuid;
}

function buildHeaders() {
  return {
    'X-API-Key':    __ENV.API_KEY || 'dev-loadtest-key',
    'X-Request-ID': `cache-${Date.now()}-${randomString(6)}`,
  };
}

export default function () {
  const uuid = nextAgent();
  const url = `${BASE_URL}/v1/agents/${uuid}`;
  const headers = buildHeaders();

  const start = Date.now();
  const res = http.get(url, { headers, tags: { name: 'agent_lookup' } });
  const duration = Date.now() - start;

  lookupLatency.add(duration);

  const success = check(res, {
    'status is 200 or 404':  r => r.status === 200 || r.status === 404,
    'latency < 5ms':         r => duration < 5,
    'no server error':       r => r.status < 500,
  });

  if (res.status === 200) {
    cacheHit.add(1);
  } else {
    cacheMiss.add(1);
  }

  if (!success) {
    errorRate.add(true);
  } else {
    errorRate.add(false);
  }
}

export function setup() {
  console.log(`==> Cache hit load test: ${BASE_URL}/v1/agents/{uuid}`);
  // 预热缓存：先访问所有热点 agent
  console.log('  - 预热 100 个热点 agent 到缓存...');
  for (const uuid of HOT_AGENTS) {
    http.get(`${BASE_URL}/v1/agents/${uuid}`, {
      headers: { 'X-API-Key': __ENV.API_KEY || 'dev-loadtest-key' },
    });
  }
  return { startTime: Date.now(), hotCount: HOT_AGENTS.length };
}

export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`==> Cache load test completed in ${duration.toFixed(1)}s (${data.hotCount} hot agents)`);
}

export function handleSummary(data) {
  return {
    'stdout': cacheTextSummary(data),
    'tests/load/results/cache-summary.json': JSON.stringify(data, null, 2),
  };
}

function cacheTextSummary(data) {
  const m = data.metrics;
  const hit = m.cache_hit_total?.values?.count || 0;
  const miss = m.cache_miss_total?.values?.count || 0;
  const ratio = hit + miss > 0 ? (hit / (hit + miss) * 100).toFixed(1) : '0.0';
  return [
    '',
    '========== Cache Hit Load Summary ==========',
    `Total requests:     ${m.http_reqs?.values?.count || 0}`,
    `HTTP failed:        ${((m.http_req_failed?.values?.rate || 0) * 100).toFixed(3)}%`,
    `P50 latency:        ${(m.http_req_duration?.values?.['p(50)'] || 0).toFixed(2)}ms`,
    `P95 latency:        ${(m.http_req_duration?.values?.['p(95)'] || 0).toFixed(2)}ms`,
    `P99 latency:        ${(m.http_req_duration?.values?.['p(99)'] || 0).toFixed(2)}ms`,
    `Cache hits:         ${hit}`,
    `Cache misses:       ${miss}`,
    `Hit ratio:          ${ratio}%`,
    `Error rate:         ${((m.cache_error_rate?.values?.rate || 0) * 100).toFixed(3)}%`,
    '============================================',
    '',
  ].join('\n');
}
