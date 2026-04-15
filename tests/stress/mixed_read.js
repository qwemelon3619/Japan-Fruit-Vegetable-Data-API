import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://jp-vgfr-api.seungpyo.xyz';
const ITEM_CODE = __ENV.ITEM_CODE || '30100';
const TEST_DATE = __ENV.TEST_DATE || '2026-04-10';
const FROM_DATE = __ENV.FROM_DATE || '2026-03-01';
const TO_DATE = __ENV.TO_DATE || '2026-04-01';

export const options = {
  stages: [
    { duration: '2m', target: 10 },
    { duration: '5m', target: 15 },
    { duration: '5m', target: 15 },
    { duration: '2m', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(95)<2500'],
  },
};

const paths = [
  '/v1/coverage',
  `/v1/prices/latest?item_code=${ITEM_CODE}&limit=20`,
  `/v1/prices/trend?item_code=${ITEM_CODE}&from=${FROM_DATE}&to=${TO_DATE}`,
  `/v1/prices/summary?item_code=${ITEM_CODE}&group_by=month&from=${FROM_DATE}&to=${TO_DATE}`,
  `/v1/rankings/items?date=${TEST_DATE}&metric=arrival&limit=20`,
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
