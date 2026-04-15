import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://jp-vgfr-api.seungpyo.xyz';
const ITEM_CODES = (__ENV.ITEM_CODES || __ENV.ITEM_CODE || '30100,30300,30600,31000,31500,32000,32500,33000').split(',');
const START_DATE = __ENV.START_DATE || '2026-04-10';
const LIMIT = __ENV.LIMIT || '20';
const TIME_UNIT = __ENV.TIME_UNIT || '1s';

function stage(target) {
  return { duration: '15s', target: Number(target) };
}

export const options = {
  scenarios: {
    breakpoint: {
      executor: 'ramping-arrival-rate',
      timeUnit: TIME_UNIT,
      preAllocatedVUs: Number(__ENV.PRE_ALLOCATED_VUS || '60'),
      maxVUs: Number(__ENV.MAX_VUS || '240'),
      stages: [
        stage(__ENV.STAGE_1_TARGET || '10'),
        stage(__ENV.STAGE_2_TARGET || '15'),
        stage(__ENV.STAGE_3_TARGET || '20'),
        stage(__ENV.STAGE_4_TARGET || '25'),
        stage(__ENV.STAGE_5_TARGET || '30'),
        stage(__ENV.STAGE_6_TARGET || '0'),
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<1000'],
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

function buildPath(iteration) {
  const dayOffset = iteration;
  const itemCode = pickByIndex(ITEM_CODES, iteration);
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

  return pickByIndex(paths, iteration);
}

export default function () {
  const path = buildPath(__ITER);
  const res = http.get(`${BASE_URL}${path}`, {
    headers: { 'Cache-Control': 'no-cache' },
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response has data': (r) => !!r.body && r.body.includes('"data"'),
  });
}
