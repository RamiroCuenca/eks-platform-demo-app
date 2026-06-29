// HTTP/CPU load profile: drives /db and /cache to push CPU past the HPA target so the
// app scales out and Karpenter provisions nodes. Run: k6 run loadtest/http.js
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '1m', target: 50 },
    { duration: '3m', target: 200 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1500'],
  },
};

export default function () {
  const db = http.get(`${BASE}/db`);
  check(db, { '/db 200': (r) => r.status === 200 });

  const cache = http.get(`${BASE}/cache`);
  check(cache, { '/cache 200': (r) => r.status === 200 });

  sleep(0.1);
}
