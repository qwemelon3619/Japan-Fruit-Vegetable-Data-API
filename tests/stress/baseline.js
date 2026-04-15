import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://jp-vgfr-api.seungpyo.xyz';
const ITEM_CODE = __ENV.ITEM_CODE || '30100';

export const options = {
  vus: 5,
  duration: '2m',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
};

const paths = [
  '/v1/coverage',
  '/v1/items?limit=10',
  `/v1/prices/latest?item_code=${ITEM_CODE}&limit=10`,
];

export default function () {
  const path = paths[Math.floor(Math.random() * paths.length)];
  const res = http.get(`${BASE_URL}${path}`);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response has data': (r) => r.body.includes('"data"'),
  });

  sleep(1);
}
