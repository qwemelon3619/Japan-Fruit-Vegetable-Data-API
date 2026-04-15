import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://jp-vgfr-api.seungpyo.xyz';
const ITEM_CODES = (__ENV.ITEM_CODES || __ENV.ITEM_CODE || '30100,30300,30600,31000').split(',');
const START_DATE = __ENV.START_DATE || '2026-03-01';

export const options = {
  vus: 5,
  duration: '2m',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
};

function addDays(dateStr, days) {
  const d = new Date(dateStr + 'T00:00:00Z');
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

function pickByIndex(arr, idx) {
  return arr[idx % arr.length];
}

export default function () {
  const dayOffset = __ITER;
  const itemCode = pickByIndex(ITEM_CODES, __ITER);
  const date = addDays(START_DATE, dayOffset);
  const from = addDays(date, -30);
  const itemQuery = itemCode.slice(0, Math.min(3, itemCode.length));

  const paths = [
    '/v1/coverage',
    `/v1/items?limit=10&q=${itemQuery}`,
    `/v1/prices/latest?item_code=${itemCode}&limit=10`,
    `/v1/prices/daily?item_code=${itemCode}&from=${from}&to=${date}&limit=10`,
  ];

  const path = pickByIndex(paths, __ITER);
  const res = http.get(`${BASE_URL}${path}`, {
    headers: { 'Cache-Control': 'no-cache' },
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response has data': (r) => !!r.body && r.body.includes('"data"'),
  });

  sleep(1);
}
