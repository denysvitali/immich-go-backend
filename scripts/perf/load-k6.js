/**
 * Optional k6 load script (not required for default Makefile / CI).
 *
 * Install: https://k6.io/docs/get-started/installation/
 * Run:
 *   k6 run -e IMMICH_URL=http://localhost:3001 scripts/perf/load-k6.js
 *   k6 run -e IMMICH_URL=http://localhost:3001 --vus 20 --duration 30s scripts/perf/load-k6.js
 *
 * Read-only smoke: health + public server metadata endpoints only.
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const baseURL = (__ENV.IMMICH_URL || 'http://localhost:3001').replace(/\/$/, '');
const failRate = new Rate('failed_requests');

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '15s',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<500'],
    failed_requests: ['rate<0.05'],
  },
};

const paths = [
  '/health',
  '/api/server/ping',
  '/api/server/version',
  '/api/server/features',
  '/api/server/config',
  '/api/server/media-types',
];

export default function () {
  const path = paths[Math.floor(Math.random() * paths.length)];
  const res = http.get(`${baseURL}${path}`, {
    tags: { name: path },
    timeout: '5s',
  });

  const ok = check(res, {
    'status is 2xx': (r) => r.status >= 200 && r.status < 300,
  });
  failRate.add(!ok);

  sleep(Number(__ENV.THINK_TIME || 0.05));
}
