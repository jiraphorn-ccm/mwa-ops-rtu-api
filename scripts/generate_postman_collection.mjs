#!/usr/bin/env node
/** Generate Postman Collection v2.1 from the RTU API route inventory. */
import { writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dir = dirname(fileURLToPath(import.meta.url));
const OUT = join(__dir, "..", "postman", "RTU-API.postman_collection.json");

const COLLECTION_VARS = [
  ["base_url", "http://127.0.0.1:5020"],
  ["api_prefix", "/api/rtu/v1"],
  ["panel_id", ""],
  ["panel_code", "PNL-DEMO"],
  ["device_model_id", ""],
  ["device_model_code", "DM-DEMO"],
  ["panel_device_id", ""],
  ["instrument_id", ""],
  ["calibration_id", ""],
  ["reading_id", ""],
  ["image_id", ""],
];

const API_TEST = `pm.test('HTTP success', () => pm.expect(pm.response.code).to.be.oneOf([200, 201]));
pm.test('MWA envelope', () => {
    const json = pm.response.json();
    pm.expect(json.status).to.eql('success');
    pm.expect(json.code).to.match(/^S\\d/);
});`;

const ROOT_TEST = `pm.test('HTTP 200', () => pm.response.to.have.status(200));
pm.test('Service root', () => {
    const json = pm.response.json();
    pm.expect(json.name).to.eql('RTU API');
    pm.expect(json.api_prefix).to.eql('/api/rtu/v1');
});`;

const HEALTH_TEST = `pm.test('HTTP success', () => pm.expect(pm.response.code).to.be.oneOf([200, 503]));
pm.test('Health JSON', () => {
    const json = pm.response.json();
    pm.expect(json.status).to.be.oneOf(['ok', 'degraded']);
});`;

const LIVE_TEST = `pm.test('HTTP 200', () => pm.response.to.have.status(200));
pm.test('Liveness', () => pm.expect(pm.response.json().status).to.eql('ok'));`;

const METRICS_TEST = `pm.test('HTTP 200', () => pm.response.to.have.status(200));
pm.test('Prometheus text', () => {
    pm.expect(pm.response.text()).to.match(/http_requests_total|HELP/);
});`;

const PREREQUEST = `if (!pm.request.headers.has('Content-Type') && pm.request.body && pm.request.body.mode === 'raw') {
    pm.request.headers.upsert({ key: 'Content-Type', value: 'application/json' });
}`;

const SAVE_ID = {
  panel_id: `if (pm.response.code === 201) {
    const d = pm.response.json().data;
    if (d?.id) pm.collectionVariables.set('panel_id', d.id);
    if (d?.code) pm.collectionVariables.set('panel_code', d.code);
}`,
  device_model_id: `if (pm.response.code === 201) {
    const d = pm.response.json().data;
    if (d?.id) pm.collectionVariables.set('device_model_id', d.id);
    if (d?.code) pm.collectionVariables.set('device_model_code', d.code);
}`,
  panel_device_id: `if (pm.response.code === 201 && pm.response.json().data?.id) {
    pm.collectionVariables.set('panel_device_id', pm.response.json().data.id);
}`,
  instrument_id: `if (pm.response.code === 201 && pm.response.json().data?.id) {
    pm.collectionVariables.set('instrument_id', pm.response.json().data.id);
}`,
  calibration_id: `if (pm.response.code === 201 && pm.response.json().data?.id) {
    pm.collectionVariables.set('calibration_id', pm.response.json().data.id);
}`,
  reading_id: `if (pm.response.code === 201 && pm.response.json().data?.id) {
    pm.collectionVariables.set('reading_id', pm.response.json().data.id);
}`,
  image_id: `if (pm.response.code === 201 && pm.response.json().data?.id) {
    pm.collectionVariables.set('image_id', pm.response.json().data.id);
}`,
};

function urlPath(segments) {
  const path = segments.filter(Boolean);
  if (path.length === 0) {
    return { raw: "{{base_url}}/", host: ["{{base_url}}"], path: [] };
  }
  const raw =
    path[0] === "{{api_prefix}}"
      ? "{{base_url}}{{api_prefix}}" +
        (path.length > 1 ? "/" + path.slice(1).join("/") : "")
      : "{{base_url}}/" + path.join("/");
  return { raw, host: ["{{base_url}}"], path };
}

function buildRequest(method, segments, { body, formdata, query } = {}) {
  const request = {
    method,
    header: [],
    url: urlPath(segments),
  };
  if (formdata) {
    request.body = { mode: "formdata", formdata };
  } else if (body !== undefined) {
    request.body = {
      mode: "raw",
      raw: JSON.stringify(body, null, 2),
      options: { raw: { language: "json" } },
    };
  }
  if (query) request.url.query = query;
  return request;
}

function example(name, method, segments, opts = {}) {
  const { body, formdata, code = 200, responseBody = "" } = opts;
  return {
    name,
    originalRequest: buildRequest(method, segments, { body, formdata }),
    status: code === 201 ? "Created" : "OK",
    code,
    _postman_previewlanguage: "json",
    header: [{ key: "Content-Type", value: "application/json" }],
    body: responseBody,
  };
}

function req(name, method, segments, opts = {}) {
  const {
    body,
    formdata,
    query,
    saveVar,
    desc = "",
    testKind = "api",
    examples = [],
  } = opts;
  const item = {
    name,
    request: buildRequest(method, segments, { body, formdata, query }),
    description: desc,
  };
  if (examples.length) {
    item.response = examples;
  }

  const tests = {
    api: API_TEST,
    root: ROOT_TEST,
    health: HEALTH_TEST,
    live: LIVE_TEST,
    metrics: METRICS_TEST,
  };
  let testScript = tests[testKind] || API_TEST;
  if (saveVar && SAVE_ID[saveVar]) testScript += "\n" + SAVE_ID[saveVar];
  item.event = [
    {
      listen: "test",
      script: { type: "text/javascript", exec: testScript.split("\n") },
    },
  ];
  return item;
}

function folder(name, items, desc = "") {
  const f = { name, item: items };
  if (desc) f.description = desc;
  return f;
}

const q = (extra = []) => [
  { key: "page", value: "1" },
  { key: "limit", value: "20" },
  { key: "sort", value: "" },
  { key: "order", value: "DESC" },
  { key: "search", value: "" },
  ...extra,
];

const api = ["{{api_prefix}}"];
const nestedReading = (suffix) => [
  ...api,
  "calibrations",
  "{{calibration_id}}",
  "readings",
  ...suffix,
];
const flatReading = (suffix) => [...api, "calibration-readings", ...suffix];

const IMAGE_FORM = [
  {
    key: "file",
    description: "Select a local image (jpeg/png/webp/gif, max 10 MB)",
    type: "file",
    src: [],
  },
  { key: "image_type", value: "EXTERIOR", type: "text" },
  { key: "caption", value: "Front view", type: "text" },
  { key: "sort_order", value: "0", type: "text" },
];

const IMAGE_REPLACE_FORM = [
  {
    key: "file",
    description: "New image file to replace existing",
    type: "file",
    src: [],
  },
  { key: "caption", value: "Updated photo", type: "text", disabled: true },
];

function build() {
  return {
    info: {
      _postman_id: "rtu-api-mwa-collection",
      name: "RTU API",
      description: [
        "MWA RTU calibration REST API.",
        "",
        "**Setup**",
        "1. Set `base_url` (default `http://127.0.0.1:5020`, no trailing slash)",
        "2. Server: `AUTH_ENABLED=false` during development (no Bearer token needed)",
        "3. Run **01 — Smoke Flow** to populate collection variables",
        "",
        "Alternate URLs for the same handler are saved as **Examples** on the primary request.",
        "Health routes return raw JSON. API routes use the MWA envelope.",
      ].join("\n"),
      schema:
        "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
    },
    event: [
      {
        listen: "prerequest",
        script: { type: "text/javascript", exec: PREREQUEST.split("\n") },
      },
    ],
    variable: COLLECTION_VARS.map(([key, value]) => ({ key, value })),
    item: [
      folder(
        "00 — Health & Info",
        [
          req("GET /", "GET", [], { testKind: "root", desc: "Service root." }),
          req("GET /health", "GET", ["health"], { testKind: "health" }),
          req("GET /health/live", "GET", ["health", "live"], {
            testKind: "live",
          }),
          req("GET /health/ready", "GET", ["health", "ready"], {
            testKind: "health",
          }),
          req("GET /metrics", "GET", ["metrics"], {
            testKind: "metrics",
            desc: "Prometheus text format when METRICS_ENABLED=true.",
          }),
        ],
        "No auth required while AUTH_ENABLED=false.",
      ),
      folder("01 — Smoke Flow (run in order)", [
        req("1. Create Panel", "POST", [...api, "panels"], {
          body: {
            code: "{{panel_code}}",
            location: "Pump Station 1",
            latitude: 13.8622,
            longitude: 100.5601,
            active: true,
          },
          saveVar: "panel_id",
        }),
        req("2. Create Device Model", "POST", [...api, "device-models"], {
          body: {
            code: "{{device_model_code}}",
            name: "RTU Controller",
            manufacturer: "Siemens",
            model: "S7-1200",
          },
          saveVar: "device_model_id",
        }),
        req(
          "3. Create Panel Device",
          "POST",
          [...api, "panels", "{{panel_id}}", "devices"],
          {
            body: {
              device_model_id: "{{device_model_id}}",
              tag_name: "FT-101",
              serial_number: "SN-DEMO-001",
              installed_at: "2025-01-15",
              communication_status: "ONLINE",
              health_status: "NORMAL",
            },
            saveVar: "panel_device_id",
          },
        ),
        req(
          "4. Create Instrument",
          "POST",
          [...api, "calibration-instruments"],
          {
            body: {
              name: "Fluke 754",
              manufacturer: "Fluke",
              serial_number: "INS-DEMO-001",
              calibration_date: "2026-01-10",
              expire_date: "2027-01-10",
            },
            saveVar: "instrument_id",
          },
        ),
        req("5. Create Calibration", "POST", [...api, "calibrations"], {
          body: {
            panel_device_id: "{{panel_device_id}}",
            instrument_id: "{{instrument_id}}",
            performed_by: "Somchai",
            performed_at: "2026-08-01T09:30:00+07:00",
            result: "PASS",
            readings: [
              {
                item_label: "Zero",
                parameter_key: "pressure",
                value: 0.02,
                unit: "bar",
              },
              {
                item_label: "Span",
                parameter_key: "pressure",
                value: 9.98,
                unit: "bar",
              },
            ],
          },
          saveVar: "calibration_id",
        }),
        req("6. Get Calibration", "GET", [
          ...api,
          "calibrations",
          "{{calibration_id}}",
        ]),
        req("7. Summary", "GET", [...api, "calibrations", "summary"]),
      ]),
      folder("API Root", [
        req("GET /api/rtu/v1/", "GET", [...api], { desc: "Service root under API prefix." }),
      ]),
      folder("Panels", [
        req("List", "GET", [...api, "panels"], {
          query: q([{ key: "active", value: "" }]),
        }),
        req("Create", "POST", [...api, "panels"], {
          body: { code: "PNL-NEW", location: "Site A" },
          saveVar: "panel_id",
        }),
        req("Get by Code", "GET", [...api, "panels", "code", "{{panel_code}}"]),
        req("Get by ID", "GET", [...api, "panels", "{{panel_id}}"]),
        req("Update PATCH", "PATCH", [...api, "panels", "{{panel_id}}"], {
          body: { location: "Building A" },
        }),
        req("Update PUT", "PUT", [...api, "panels", "{{panel_id}}"], {
          body: { location: "Building B" },
        }),
        req("Soft Delete", "DELETE", [...api, "panels", "{{panel_id}}"]),
        req("Restore", "POST", [...api, "panels", "{{panel_id}}", "restore"]),
        req("Hard Delete", "DELETE", [
          ...api,
          "panels",
          "{{panel_id}}",
          "permanent",
        ]),
        req(
          "List Devices",
          "GET",
          [...api, "panels", "{{panel_id}}", "devices"],
          { query: q() },
        ),
        req(
          "Create Device",
          "POST",
          [...api, "panels", "{{panel_id}}", "devices"],
          {
            body: {
              device_model_id: "{{device_model_id}}",
              tag_name: "FT-102",
            },
            saveVar: "panel_device_id",
          },
        ),
      ]),
      folder(
        "Panel Images",
        [
          req("List", "GET", [...api, "panels", "{{panel_id}}", "images"], {
            query: q([{ key: "image_type", value: "" }]),
            desc: "List panel photos. Each item includes presigned `url`.",
          }),
          req("Upload", "POST", [...api, "panels", "{{panel_id}}", "images"], {
            formdata: IMAGE_FORM,
            saveVar: "image_id",
            desc: "multipart/form-data — `file` and `image_type` required.",
          }),
          req(
            "Get by ID",
            "GET",
            [...api, "panels", "{{panel_id}}", "images", "{{image_id}}"],
            { desc: "Detail with presigned `url`." },
          ),
          req(
            "Update PATCH",
            "PATCH",
            [...api, "panels", "{{panel_id}}", "images", "{{image_id}}"],
            {
              body: { caption: "Updated caption", sort_order: 1 },
              desc: "Partial metadata update (JSON).",
            },
          ),
          req(
            "Update PUT",
            "PUT",
            [...api, "panels", "{{panel_id}}", "images", "{{image_id}}"],
            {
              body: {
                image_type: "EXTERIOR",
                sort_order: 0,
                caption: "Full replace caption",
              },
              desc: "Full metadata replace (JSON). Use example for multipart file replace.",
              examples: [
                example(
                  "Replace file (multipart)",
                  "PUT",
                  [...api, "panels", "{{panel_id}}", "images", "{{image_id}}"],
                  { formdata: IMAGE_REPLACE_FORM },
                ),
              ],
            },
          ),
          req(
            "Delete",
            "DELETE",
            [...api, "panels", "{{panel_id}}", "images", "{{image_id}}"],
            { desc: "Permanent delete — removes DB row and S3 object." },
          ),
        ],
        "Panel photos stored in S3. Requires S3_BUCKET in server .env.",
      ),
      folder("Device Models", [
        req("List", "GET", [...api, "device-models"], {
          query: q([{ key: "manufacturer", value: "" }]),
        }),
        req("Create", "POST", [...api, "device-models"], {
          body: { code: "DM-NEW", name: "Controller" },
          saveVar: "device_model_id",
        }),
        req("Get by Code", "GET", [
          ...api,
          "device-models",
          "code",
          "{{device_model_code}}",
        ]),
        req("Get by ID", "GET", [
          ...api,
          "device-models",
          "{{device_model_id}}",
        ]),
        req(
          "Update PATCH",
          "PATCH",
          [...api, "device-models", "{{device_model_id}}"],
          { body: { description: "x" } },
        ),
        req(
          "Update PUT",
          "PUT",
          [...api, "device-models", "{{device_model_id}}"],
          { body: { description: "y" } },
        ),
        req("Soft Delete", "DELETE", [
          ...api,
          "device-models",
          "{{device_model_id}}",
        ]),
        req("Restore", "POST", [
          ...api,
          "device-models",
          "{{device_model_id}}",
          "restore",
        ]),
        req("Hard Delete", "DELETE", [
          ...api,
          "device-models",
          "{{device_model_id}}",
          "permanent",
        ]),
      ]),
      folder("Panel Devices", [
        req("List", "GET", [...api, "panel-devices"], {
          query: q([
            { key: "panel_id", value: "{{panel_id}}" },
            { key: "communication_status", value: "" },
            { key: "health_status", value: "" },
          ]),
        }),
        req("Create", "POST", [...api, "panel-devices"], {
          body: {
            panel_id: "{{panel_id}}",
            device_model_id: "{{device_model_id}}",
            tag_name: "FT-200",
          },
          saveVar: "panel_device_id",
        }),
        req("Get by ID", "GET", [
          ...api,
          "panel-devices",
          "{{panel_device_id}}",
        ]),
        req(
          "Update PATCH",
          "PATCH",
          [...api, "panel-devices", "{{panel_device_id}}"],
          { body: { note: "ok" } },
        ),
        req(
          "Update PUT",
          "PUT",
          [...api, "panel-devices", "{{panel_device_id}}"],
          { body: { note: "ok2" } },
        ),
        req(
          "Record Status",
          "POST",
          [...api, "panel-devices", "{{panel_device_id}}", "status"],
          {
            body: {
              communication_status: "DEGRADED",
              health_status: "WARNING",
            },
          },
        ),
        req(
          "List Calibrations",
          "GET",
          [...api, "panel-devices", "{{panel_device_id}}", "calibrations"],
          { query: q() },
        ),
        req(
          "Create Calibration",
          "POST",
          [...api, "panel-devices", "{{panel_device_id}}", "calibrations"],
          {
            body: {
              instrument_id: "{{instrument_id}}",
              performed_by: "Somchai",
              performed_at: "2026-08-01T09:30:00+07:00",
              result: "PASS",
            },
            saveVar: "calibration_id",
          },
        ),
        req("Soft Delete", "DELETE", [
          ...api,
          "panel-devices",
          "{{panel_device_id}}",
        ]),
        req("Restore", "POST", [
          ...api,
          "panel-devices",
          "{{panel_device_id}}",
          "restore",
        ]),
        req("Hard Delete", "DELETE", [
          ...api,
          "panel-devices",
          "{{panel_device_id}}",
          "permanent",
        ]),
      ]),
      folder("Calibration Instruments", [
        req("List", "GET", [...api, "calibration-instruments"], {
          query: q([{ key: "expired", value: "" }]),
        }),
        req("Create", "POST", [...api, "calibration-instruments"], {
          body: {
            name: "Fluke 754",
            calibration_date: "2026-01-10",
            expire_date: "2027-01-10",
          },
          saveVar: "instrument_id",
        }),
        req("Get by ID", "GET", [
          ...api,
          "calibration-instruments",
          "{{instrument_id}}",
        ]),
        req(
          "Update PATCH",
          "PATCH",
          [...api, "calibration-instruments", "{{instrument_id}}"],
          { body: { name: "Fluke" } },
        ),
        req(
          "Update PUT",
          "PUT",
          [...api, "calibration-instruments", "{{instrument_id}}"],
          { body: { name: "Fluke" } },
        ),
        req("Soft Delete", "DELETE", [
          ...api,
          "calibration-instruments",
          "{{instrument_id}}",
        ]),
        req("Restore", "POST", [
          ...api,
          "calibration-instruments",
          "{{instrument_id}}",
          "restore",
        ]),
        req("Hard Delete", "DELETE", [
          ...api,
          "calibration-instruments",
          "{{instrument_id}}",
          "permanent",
        ]),
      ]),
      folder("Calibrations", [
        req("List", "GET", [...api, "calibrations"], {
          query: q([{ key: "result", value: "" }]),
        }),
        req("Summary", "GET", [...api, "calibrations", "summary"]),
        req("Create", "POST", [...api, "calibrations"], {
          body: {
            panel_device_id: "{{panel_device_id}}",
            instrument_id: "{{instrument_id}}",
            performed_by: "Somchai",
            performed_at: "2026-08-01T09:30:00+07:00",
            result: "PASS",
          },
          saveVar: "calibration_id",
        }),
        req("Get by ID", "GET", [...api, "calibrations", "{{calibration_id}}"]),
        req(
          "Update PATCH",
          "PATCH",
          [...api, "calibrations", "{{calibration_id}}"],
          { body: { remark: "x" } },
        ),
        req(
          "Update PUT",
          "PUT",
          [...api, "calibrations", "{{calibration_id}}"],
          { body: { remark: "y" } },
        ),
        req("Delete", "DELETE", [...api, "calibrations", "{{calibration_id}}"]),
        req("List Readings", "GET", nestedReading([])),
        req("Add Reading", "POST", nestedReading([]), {
          body: { parameter_key: "humidity", value: 60, unit: "%" },
          saveVar: "reading_id",
        }),
        req("Replace Readings", "PUT", nestedReading([]), {
          body: {
            readings: [
              { parameter_key: "temperature", value: 25.5, unit: "C" },
            ],
          },
        }),
        req("Get Reading", "GET", nestedReading(["{{reading_id}}"]), {
          desc: "Nested route. Flat `/calibration-readings/{id}` is in Examples.",
          examples: [
            example("Flat route", "GET", flatReading(["{{reading_id}}"])),
          ],
        }),
        req("Update Reading PATCH", "PATCH", nestedReading(["{{reading_id}}"]), {
          body: { value: 0.01 },
          examples: [
            example("Flat route", "PATCH", flatReading(["{{reading_id}}"]), {
              body: { value: 0.01 },
            }),
          ],
        }),
        req("Update Reading PUT", "PUT", nestedReading(["{{reading_id}}"]), {
          body: { parameter_key: "pressure", value: 0.02, unit: "bar" },
          examples: [
            example("Flat route", "PUT", flatReading(["{{reading_id}}"]), {
              body: { parameter_key: "pressure", value: 0.02, unit: "bar" },
            }),
          ],
        }),
        req("Delete Reading", "DELETE", nestedReading(["{{reading_id}}"]), {
          examples: [
            example("Flat route", "DELETE", flatReading(["{{reading_id}}"])),
          ],
        }),
      ]),
    ],
  };
}

function countRequests(items) {
  return items.reduce(
    (n, item) => n + (item.item ? countRequests(item.item) : 1),
    0,
  );
}

const collection = build();
mkdirSync(dirname(OUT), { recursive: true });
writeFileSync(OUT, JSON.stringify(collection, null, 2) + "\n", "utf8");
console.log(`Wrote ${OUT}`);
console.log(`Requests: ${countRequests(collection.item)}`);
