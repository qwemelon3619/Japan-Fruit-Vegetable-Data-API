import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://jp-vgfr-api.seungpyo.xyz';
const ITEM_CODES = (__ENV.ITEM_CODES || __ENV.ITEM_CODE || '30100,30300,30600,31000,31500,32000,32500,33000').split(',');
const START_DATE = __ENV.START_DATE || '2025-04-10';
const LIMIT = __ENV.LIMIT || '20';

function stage(duration, target) {
  return { duration, target: Number(target) };
}

export const options = {
  stages: [
    stage(__ENV.STAGE_1_DURATION || '2m', __ENV.STAGE_1_TARGET || '20'),
    stage(__ENV.STAGE_2_DURATION || '3m', __ENV.STAGE_2_TARGET || '40'),
    stage(__ENV.STAGE_3_DURATION || '3m', __ENV.STAGE_3_TARGET || '80'),
    stage(__ENV.STAGE_4_DURATION || '3m', __ENV.STAGE_4_TARGET || '120'),
    stage(__ENV.STAGE_5_DURATION || '3m', __ENV.STAGE_5_TARGET || '160'),
    stage(__ENV.STAGE_6_DURATION || '2m', __ENV.STAGE_6_TARGET || '0'),
  ],
  thresholds: {
    http_req_failed: ['rate<0.10'],
    http_req_duration: ['p(95)<5000'],
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
  const from = addDays(date, -31);
  const to = date;

  const paths = [
    `/v1/prices/latest?item_code=${itemCode}&limit=${LIMIT}`,
    `/v1/prices/daily?item_code=${itemCode}&from=${from}&to=${to}&limit=${LIMIT}`,
    `/v1/prices/trend?item_code=${itemCode}&from=${from}&to=${to}`,
    `/v1/prices/summary?item_code=${itemCode}&group_by=month&from=${from}&to=${to}`,
    `/v1/rankings/items?date=${date}&metric=arrival&limit=${LIMIT}`,
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
