import http from "k6/http";
import { check, sleep } from "k6";

// Learning load script for redirect hot path. Run against local shortener only.
export const options = {
  vus: 5,
  duration: "10s",
  thresholds: {
    http_req_failed: ["rate<0.05"],
  },
};

const base = __ENV.SHORTENER_BASE || "http://localhost:8094";
const code = __ENV.SHORTENER_CODE || "demo";

export default function () {
  const res = http.get(`${base}/${code}`, { redirects: 0 });
  check(res, {
    "redirect or rate limit": (r) => r.status === 302 || r.status === 404 || r.status === 429,
  });
  sleep(0.2);
}
