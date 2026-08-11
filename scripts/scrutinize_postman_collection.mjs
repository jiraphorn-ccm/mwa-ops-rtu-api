#!/usr/bin/env node
/** Scrutinize: verify Postman collection covers every route in router.go */
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const collection = JSON.parse(
  readFileSync(join(root, "postman", "RTU-API.postman_collection.json"), "utf8"),
);

const EXPECTED = [
  ["GET", "/"],
  ["GET", "/health"],
  ["GET", "/health/live"],
  ["GET", "/health/ready"],
  ["GET", "/metrics"],
  ["GET", "{{api_prefix}}/"],
  ["GET", "{{api_prefix}}/panels"],
  ["POST", "{{api_prefix}}/panels"],
  ["GET", "{{api_prefix}}/panels/code/{{panel_code}}"],
  ["GET", "{{api_prefix}}/panels/{{panel_id}}"],
  ["PUT", "{{api_prefix}}/panels/{{panel_id}}"],
  ["PATCH", "{{api_prefix}}/panels/{{panel_id}}"],
  ["DELETE", "{{api_prefix}}/panels/{{panel_id}}"],
  ["DELETE", "{{api_prefix}}/panels/{{panel_id}}/permanent"],
  ["POST", "{{api_prefix}}/panels/{{panel_id}}/restore"],
  ["GET", "{{api_prefix}}/panels/{{panel_id}}/devices"],
  ["POST", "{{api_prefix}}/panels/{{panel_id}}/devices"],
  ["GET", "{{api_prefix}}/panels/{{panel_id}}/images"],
  ["POST", "{{api_prefix}}/panels/{{panel_id}}/images"],
  ["GET", "{{api_prefix}}/panels/{{panel_id}}/images/{{image_id}}"],
  ["PUT", "{{api_prefix}}/panels/{{panel_id}}/images/{{image_id}}"],
  ["PATCH", "{{api_prefix}}/panels/{{panel_id}}/images/{{image_id}}"],
  ["DELETE", "{{api_prefix}}/panels/{{panel_id}}/images/{{image_id}}"],
  ["GET", "{{api_prefix}}/device-models"],
  ["POST", "{{api_prefix}}/device-models"],
  ["GET", "{{api_prefix}}/device-models/code/{{device_model_code}}"],
  ["GET", "{{api_prefix}}/device-models/{{device_model_id}}"],
  ["PUT", "{{api_prefix}}/device-models/{{device_model_id}}"],
  ["PATCH", "{{api_prefix}}/device-models/{{device_model_id}}"],
  ["DELETE", "{{api_prefix}}/device-models/{{device_model_id}}"],
  ["DELETE", "{{api_prefix}}/device-models/{{device_model_id}}/permanent"],
  ["POST", "{{api_prefix}}/device-models/{{device_model_id}}/restore"],
  ["GET", "{{api_prefix}}/panel-devices"],
  ["POST", "{{api_prefix}}/panel-devices"],
  ["GET", "{{api_prefix}}/panel-devices/{{panel_device_id}}"],
  ["PUT", "{{api_prefix}}/panel-devices/{{panel_device_id}}"],
  ["PATCH", "{{api_prefix}}/panel-devices/{{panel_device_id}}"],
  ["DELETE", "{{api_prefix}}/panel-devices/{{panel_device_id}}"],
  ["DELETE", "{{api_prefix}}/panel-devices/{{panel_device_id}}/permanent"],
  ["POST", "{{api_prefix}}/panel-devices/{{panel_device_id}}/restore"],
  ["POST", "{{api_prefix}}/panel-devices/{{panel_device_id}}/status"],
  ["GET", "{{api_prefix}}/panel-devices/{{panel_device_id}}/calibrations"],
  ["POST", "{{api_prefix}}/panel-devices/{{panel_device_id}}/calibrations"],
  ["GET", "{{api_prefix}}/calibration-instruments"],
  ["POST", "{{api_prefix}}/calibration-instruments"],
  ["GET", "{{api_prefix}}/calibration-instruments/{{instrument_id}}"],
  ["PUT", "{{api_prefix}}/calibration-instruments/{{instrument_id}}"],
  ["PATCH", "{{api_prefix}}/calibration-instruments/{{instrument_id}}"],
  ["DELETE", "{{api_prefix}}/calibration-instruments/{{instrument_id}}"],
  ["DELETE", "{{api_prefix}}/calibration-instruments/{{instrument_id}}/permanent"],
  ["POST", "{{api_prefix}}/calibration-instruments/{{instrument_id}}/restore"],
  ["GET", "{{api_prefix}}/calibrations"],
  ["POST", "{{api_prefix}}/calibrations"],
  ["GET", "{{api_prefix}}/calibrations/summary"],
  ["GET", "{{api_prefix}}/calibrations/{{calibration_id}}"],
  ["PUT", "{{api_prefix}}/calibrations/{{calibration_id}}"],
  ["PATCH", "{{api_prefix}}/calibrations/{{calibration_id}}"],
  ["DELETE", "{{api_prefix}}/calibrations/{{calibration_id}}"],
  ["GET", "{{api_prefix}}/calibrations/{{calibration_id}}/readings"],
  ["POST", "{{api_prefix}}/calibrations/{{calibration_id}}/readings"],
  ["PUT", "{{api_prefix}}/calibrations/{{calibration_id}}/readings"],
  ["GET", "{{api_prefix}}/calibrations/{{calibration_id}}/readings/{{reading_id}}"],
  ["PUT", "{{api_prefix}}/calibrations/{{calibration_id}}/readings/{{reading_id}}"],
  ["PATCH", "{{api_prefix}}/calibrations/{{calibration_id}}/readings/{{reading_id}}"],
  ["DELETE", "{{api_prefix}}/calibrations/{{calibration_id}}/readings/{{reading_id}}"],
  ["GET", "{{api_prefix}}/calibration-readings/{{reading_id}}"],
  ["PUT", "{{api_prefix}}/calibration-readings/{{reading_id}}"],
  ["PATCH", "{{api_prefix}}/calibration-readings/{{reading_id}}"],
  ["DELETE", "{{api_prefix}}/calibration-readings/{{reading_id}}"],
];

function normalize(raw) {
  if (!raw) return "";
  return raw
    .replace(/\{\{base_url\}\}/g, "")
    .replace(/\/+/g, "/")
    .replace(/\/$/, "") || "/";
}

function addRoute(out, method, raw, source) {
  out.push({
    method,
    path: normalize(raw),
    source,
  });
}

function collectRequests(items, out = []) {
  for (const item of items) {
    if (item.item) {
      collectRequests(item.item, out);
      continue;
    }
    const req = item.request;
    if (req?.url?.raw) {
      addRoute(out, req.method, req.url.raw, item.name);
    }
    for (const ex of item.response ?? []) {
      const orig = ex.originalRequest;
      if (orig?.url?.raw) {
        addRoute(out, orig.method, orig.url.raw, `${item.name} → ${ex.name}`);
      }
    }
  }
  return out;
}

const found = collectRequests(collection.item);
const foundSet = new Set(found.map((r) => `${r.method} ${r.path}`));

const missing = [];
for (const [method, path] of EXPECTED) {
  const key = `${method} ${normalize(path)}`;
  if (!foundSet.has(key)) missing.push(key);
}

console.log(`Expected: ${EXPECTED.length} routes`);
console.log(`Collection: ${found.length} route entries (requests + examples)`);
console.log(`Covered: ${EXPECTED.length - missing.length}/${EXPECTED.length}`);

if (missing.length) {
  console.error("MISSING routes:");
  missing.forEach((m) => console.error("  -", m));
  process.exit(1);
}

console.log("SCRUTINIZE: OK — all router.go routes present");
