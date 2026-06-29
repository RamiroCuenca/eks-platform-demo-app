// Queue load profile: floods /enqueue so the Redis list outgrows what the worker drains,
// building the backlog KEDA's Redis scaler reacts to. Run: k6 run loadtest/enqueue.js
import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '2m', target: 300 }, // build the backlog
    { duration: '2m', target: 0 },   // let the workers drain it back down
  ],
};

export default function () {
  const res = http.post(`${BASE}/enqueue`);
  check(res, { '/enqueue 202': (r) => r.status === 202 });
}
