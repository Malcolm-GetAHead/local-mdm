import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '10s', target: 50 },
    { duration: '1m', target: 100 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.05'],
  },
};

export default function () {
  // Simulate Windows discovery endpoint (unauthenticated enrollment path)
  const discovery = http.get(`${BASE_URL}/EnrollmentServer/Discovery.svc`);
  check(discovery, { 'discovery reachable': (r) => r.status < 500 });

  // Simulate SCEP GetCACaps (unauthenticated)
  const scep = http.get(`${BASE_URL}/scep?operation=GetCACaps`);
  check(scep, { 'scep reachable': (r) => r.status < 500 });

  // Health check under load
  const health = http.get(`${BASE_URL}/health`);
  check(health, { 'health 200': (r) => r.status === 200 });

  sleep(0.5 + Math.random());
}
