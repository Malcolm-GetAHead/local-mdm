import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.AUTH_TOKEN || '';

export const options = {
  stages: [
    { duration: '15s', target: 10 },
    { duration: '1m30s', target: 10 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<300'],
    http_req_failed: ['rate<0.05'],
  },
};

const headers = TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {};

export default function () {
  // List devices
  const devices = http.get(`${BASE_URL}/api/v1/devices?limit=50`, { headers });
  check(devices, { 'devices status ok': (r) => r.status < 500 });

  // List policies
  const policies = http.get(`${BASE_URL}/api/v1/policies?limit=50`, { headers });
  check(policies, { 'policies status ok': (r) => r.status < 500 });

  // Health (always accessible)
  const health = http.get(`${BASE_URL}/health`);
  check(health, { 'health 200': (r) => r.status === 200 });

  sleep(1 + Math.random());
}
