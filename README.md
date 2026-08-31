# RTU API

REST API สำหรับงาน **RTU panel / device / calibration / PM / CM** ของ MWA
เขียนด้วย Go + PostgreSQL และตอบกลับตาม envelope มาตรฐานเดียวกับ service อื่นในระบบ
(ดู [`api-response-reference.md`](./api-response-reference.md))

---

## 1. Stack

| ส่วน | เครื่องมือ | เหตุผลที่เลือก |
|------|-----------|----------------|
| ภาษา | **Go 1.25** | คอมไพล์เป็น binary เดียว, deploy ง่าย, backward compatible ระยะยาว |
| HTTP | **net/http + [chi v5](https://github.com/go-chi/chi)** | chi เป็น `http.Handler` ล้วน ๆ ไม่ผูก framework — เปลี่ยน/ถอดได้ตลอด |
| Driver | **[pgx/v5](https://github.com/jackc/pgx)** + `pgxpool` | driver PostgreSQL ที่เร็วที่สุดใน Go, ใช้ binary protocol, มี connection pool ในตัว |
| Query layer | **[sqlc](https://sqlc.dev)** | เขียน SQL จริง แล้ว generate Go ที่ type-safe — ตรวจ SQL ตั้งแต่ตอน build |
| Migration | **[golang-migrate](https://github.com/golang-migrate/migrate)** | ไฟล์ SQL ธรรมดา up/down, มี CLI ให้ใช้ตรง ๆ |
| Validation | **[validator/v10](https://github.com/go-playground/validator)** | มาตรฐาน de-facto ของ Go |
| ตัวเลข | **[shopspring/decimal](https://github.com/shopspring/decimal)** | `numeric` ของ Postgres ไม่เพี้ยนเหมือน float |
| Log | **`log/slog`** (stdlib) | structured logging ที่อยู่ใน Go เอง ไม่ต้องพึ่ง lib ภายนอก |
| Config | **caarlos0/env + godotenv** | อ่านจาก environment เป็นหลัก ตาม 12-factor |

---

## 2. โครงสร้างโปรเจกต์

```
server/
├── cmd/server/              entrypoint + graceful shutdown
├── migrations/              golang-migrate SQL (embed ไว้เช็ค schema version)
├── queries/                 SQL ต้นทางของ sqlc
├── sqlc.yaml                config การ generate
└── internal/
    ├── config/              โหลด/ตรวจ environment
    ├── db/
    │   ├── pool.go          pgxpool + register ชนิด decimal
    │   ├── tx.go            helper ทำ transaction
    │   ├── pgerr.go         แปลง SQLSTATE → error code ของ API
    │   ├── schema.go        เทียบ migration ที่ apply แล้วกับที่ฝังในไบนารี
    │   └── sqlc/            โค้ดที่ sqlc generate (ห้ามแก้มือ)
    ├── httpx/               envelope, error/success codes, bind, validate, paging
    ├── middleware/          request id, log, recover, CORS, rate limit, body limit
    ├── repository/          ชั้นเดียวที่คุยกับ PostgreSQL
    ├── service/             business rules + request DTO
    ├── handler/             แปลง HTTP ↔ service
    └── router/              ผูก URL กับ handler
```

**ทิศทางการพึ่งพา:** `handler → service → repository → sqlc/pgx`
ทุกชั้นส่ง error กลับเป็น `*httpx.AppError` ตัวเดียว handler จึงแค่เรียก `httpx.Error(w, r, err)`

### ทำไมมีทั้ง sqlc และ SQL เขียนมือ

* **sqlc** ใช้กับ insert / update / delete / lookup ด้วย id — sqlc ตรวจ SQL กับ schema จริงตอน generate
* **SQL เขียนมือใน `repository/`** ใช้เฉพาะ endpoint แบบ list ที่ต้องมี `ORDER BY` และ `WHERE` เปลี่ยนไปตาม query string
  * ชื่อคอลัมน์ที่ sort ได้มาจาก whitelist (`httpx.Sortable`) เท่านั้น
  * ค่าจาก client ทุกตัวส่งเป็น bound parameter (`$1`, `$2`, …) ไม่มีการต่อ string
  * นับ total ด้วย `count(*) OVER ()` จึงยิง query เดียวได้ทั้งข้อมูลและจำนวนแถว

---

## 3. เริ่มใช้งาน

ต้องมี PostgreSQL 17 รันอยู่แล้ว (local install, managed service ฯลฯ) — โปรเจกต์นี้ไม่ผูกกับ Docker

```bash
# สร้าง role/database (ตัวอย่างด้วย psql)
psql -U postgres -c "CREATE ROLE rtu LOGIN PASSWORD 'rtu_password';"
psql -U postgres -c "CREATE DATABASE rtu OWNER rtu;"

cp .env.example .env
# แก้ DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME ใน .env ให้ตรงกับ PostgreSQL จริง

make tools          # ติดตั้ง sqlc + migrate CLI (ครั้งแรกครั้งเดียว)
make migrate-up     # รัน schema migration
make run            # รัน API
```

เปิด http://localhost:5020/health

### Postman

| ไฟล์ | ใช้ทำอะไร |
|------|-----------|
| [`postman/RTU-API.postman_collection.json`](./postman/RTU-API.postman_collection.json) | Collection ครบทุก route (155 routes — regenerate ด้วย script ด้านล่าง) |
| [`postman/RTU-API.local.postman_environment.json`](./postman/RTU-API.local.postman_environment.json) | Environment local (`base_url`, `actor_id`, …) |

Import ทั้งสองไฟล์ใน Postman แล้วรัน folder **01 — Smoke Flow** เพื่อเติม collection variables

```bash
node scripts/generate_postman_collection.mjs   # หลังเพิ่ม route
node scripts/scrutinize_postman_collection.mjs  # ตรวจ coverage vs router.go
```

### คำสั่งที่ใช้บ่อย

```bash
make help            # ดู target ทั้งหมด
make generate        # regenerate โค้ดหลังแก้ queries/ หรือ migrations/
make test            # unit tests
make test-integration # ต้องตั้งค่า DB_* ใน .env และ build tag integration
make lint            # fmt + vet + sqlc vet
make migrate-create NAME=add_alarm_table
```

---

## 4. Database

Schema อยู่ใน PostgreSQL schema ชื่อ `rtu` — เอกสาร canonical:

| ไฟล์ | ใช้ทำอะไร |
|------|-----------|
| [`doc/rtu-full-schema.dbml`](./doc/rtu-full-schema.dbml) | ER diagram — paste ลง [dbdiagram.io](https://dbdiagram.io) |
| [`doc/rtu_db_dictionary.html`](./doc/rtu_db_dictionary.html) | Data dictionary (เปิดใน browser) — regenerate: `node scripts/generate_rtu_db_dictionary.mjs` |

Source of truth ในโค้ด: `migrations/000001`–`000008`

Regenerate dictionary: `node scripts/generate_rtu_db_dictionary.mjs`  
Regenerate Postman: `node scripts/generate_postman_collection.mjs` → `postman/RTU-API.postman_collection.json`

### ER ภาพรวม

```
panels ──< panel_devices    device_models (master catalog — ไม่มี FK)
   │              │
   │              ├──< calibrations >── calibration_instruments
   │              │         │
   │              │         └──< calibration_readings
   │              │
   │              └──< cm_reports (PM_ONSITE_FIX origin)
   │
   ├──< panel_images
   ├──< work_orders ──< work_order_rounds
   │         │                  │
   │         │                  ├──< pm_reports ──< checklist_results >── checklist_items
   │         │                  │         ├── pm_ground_tests
   │         │                  │         ├── pm_power_tests ──< pm_power_test_points
   │         │                  │         └── (calibrations ผ่าน pm_report_id / work_order_id)
   │         │                  │
   │         │                  ├──< cm_reports >── problem_topics
   │         │                  │         (STANDALONE / PM_ESCALATED)
   │         │                  └──< wo_approvals (1 ต่อ 1 ต่อ round)
   │         │
   │         ├──< work_order_activity_logs
   │         └──< notifications
   │
   └── engineers (อ้างอิงจาก pm_reports / tests)

attachments — polymorphic (WORK_ORDER, PM_REPORT, CM_REPORT, CALIBRATION,
              PM_GROUND_TEST, PM_POWER_TEST_POINT, PANEL_DEVICE)
```

### ตารางทั้งหมด (22 ตาราง)

| กลุ่ม | ตาราง | migration | หมายเหตุ |
|-------|--------|-----------|----------|
| Core RTU | `panels` | 000001 | + `install_date`, `last_pm_date`, `next_pm_date` ใน 000006 |
| | `device_models` | 000001 + 000008 | master catalog — ไม่ผูก panel_devices |
| | `panel_devices` | 000001 + 000008 | equipment snapshot + `calibration_date` |
| | `panel_images` | 000003 | S3 presigned URL |
| Calibration | `calibration_instruments` | 000001 | + `equipment_type`, `brand` ใน 000007 |
| | `calibrations` | 000001 | + PM link (`work_order_id`, `pm_report_id`, EUT fields) ใน 000006 |
| | `calibration_readings` | 000001 | |
| PM/CM master | `engineers` | 000006 | วิศวกรลงนามรายงาน PM |
| | `checklist_items` | 000006 | master checklist (PM3 / PM6) |
| | `problem_topics` | 000007 | master หัวข้อปัญหา CM (pill UI) — seed 12 รายการ |
| Work order | `work_orders` | 000006 | PM / CM, `pm_schedule_type`, `current_round_id` |
| | `work_order_rounds` | 000006 | multi-round visit / rework |
| | `work_order_activity_logs` | 000006 | ASSIGNED, CHECK_IN, CM_SPAWNED, … |
| | `wo_approvals` | 000006 | APPROVED / CONDITION / REJECTED (+ escalate CM) |
| PM report | `pm_reports` | 000006 | 1 ต่อ 1 ต่อ round |
| | `checklist_results` | 000006 | ผล checklist ต่อรายการ |
| | `pm_ground_tests` | 000006 | ทดสอบ ground (optional) |
| | `pm_power_tests` | 000006 | ทดสอบ power — **บังคับ PM 3 เดือน** |
| | `pm_power_test_points` | 000006 | breaker / DC supply แต่ละจุด |
| CM report | `cm_reports` | 000006 | 3 origins + `problem_topic_id` (000007) |
| Files / notify | `attachments` | 000006 | polymorphic upload (S3) |
| | `notifications` | 000006 | NEW_ASSIGNMENT, PENDING_APPROVAL, COMPLETED, CM_PENDING |

### PM schedule type

| `pm_schedule_type` | Checklist | Ground | Power test | Calibration |
|--------------------|-----------|--------|------------|-------------|
| `THREE_MONTH` | ✓ | optional | **required** (submit) | — |
| `SIX_MONTH` | ✓ | optional | — | **≥1 ใบ** (submit) |

`calibrations` ผูก PM 6 เดือนได้ผ่าน `work_order_id` และ/หรือ `pm_report_id` (ต้องเป็น SIX_MONTH PM)

### CM report origins

| Origin | `work_order_id` | `pm_report_id` | ใช้เมื่อ |
|--------|-----------------|----------------|----------|
| STANDALONE | ✓ | — | CM work order ปกติ |
| PM_ONSITE_FIX | — | ✓ | ซ่อมหน้างานระหว่าง PM |
| PM_ESCALATED | ✓ | ✓ | escalate จาก PM (Report issue) |

### สิ่งที่ปรับจาก DBML ตั้งต้น

| เรื่อง | เดิม | ปรับเป็น | เหตุผล |
|-------|------|---------|--------|
| `created_at` / `updated_at` | nullable | `NOT NULL DEFAULT now()` + trigger | Go ได้ `time.Time` ตรง ๆ ไม่ต้องเช็ค null และ `updated_at` ขยับเองเสมอ |
| `device_models`, `calibration_instruments` | ไม่มี timestamp | เพิ่ม `created_at` / `updated_at` | ให้ audit trail เท่ากันทุกตาราง |
| `communication_status`, `health_status` | nullable, ไม่มี CHECK | `NOT NULL DEFAULT 'UNKNOWN'` + CHECK | สถานะที่ไม่รู้ควรเป็น `UNKNOWN` ไม่ใช่ null |
| `result` | ไม่มี CHECK | CHECK `PASS / FAIL / ADJUSTED` | กันข้อมูลเพี้ยนแม้เขียนตรงเข้า DB |
| `(calibration_id, sequence)` | index ธรรมดา | **UNIQUE** | ลำดับซ้ำในใบเดียวกันคือข้อมูลผิด |
| `(panel_id, tag_name)` | ไม่มี | partial UNIQUE (`WHERE tag_name IS NOT NULL`) | tag ควรไม่ซ้ำภายในตู้เดียวกัน |
| latitude / longitude | ไม่มี CHECK | CHECK ช่วง ±90 / ±180 | กันพิกัดที่เป็นไปไม่ได้ |
| `expire_date` | ไม่มี CHECK | ต้องมากกว่า `calibration_date` | ใบรับรองหมดอายุก่อนสอบเทียบไม่ได้ |
| FK | ไม่ระบุ | `ON DELETE RESTRICT` ทุกที่ ยกเว้น readings ที่ `CASCADE` | ลบตู้/รุ่น/เครื่องมือทิ้งโดยยังมีของอ้างอิงไม่ได้ แต่ลบใบสอบเทียบแล้วค่าที่วัดควรหายตาม |

`updated_at` ขยับด้วย trigger `rtu.set_updated_at()` ไม่ใช่ที่ฝั่งแอป — เขียนผ่านช่องทางไหนก็ตรงกันเสมอ

### Soft delete

`DELETE /{resource}/{id}` ตั้ง `active = false` (ตอบ `{ "id": ..., "soft_deleted": true }`)
กู้คืนด้วย `POST /{resource}/{id}/restore`
ลบจริงต้องเรียก `DELETE /{resource}/{id}/permanent` และจะถูกปฏิเสธถ้ายังมี record อ้างอิงอยู่

`calibrations` และ `calibration_readings` ไม่มี `active` — `DELETE` คือลบจริง และ readings จะ cascade ตามใบสอบเทียบ

---

## 5. Endpoints

Base path: `{API_PREFIX}` (ค่าเริ่มต้น `/api/rtu/v1`)

### REST conventions (ทุก resource)

| ระดับ | GET | POST | PUT | PATCH | DELETE |
|-------|-----|------|-----|-------|--------|
| **Collection** `/resources` | List | Create | — | — | — |
| **Item** `/resources/{id}` | Detail | — | Full replace | Partial update | Delete |

- **PUT** = ส่ง representation ครบชุด (field ที่ส่งมาทั้งหมดจะถูก apply)
- **PATCH** = ส่งเฉพาะ field ที่ต้องการเปลี่ยน
- **POST** = สร้าง resource ใหม่เท่านั้น (ไม่ใช้ PUT ที่ collection)
- รูปภาพ: **PUT + multipart** = แทนที่ไฟล์ · **PUT/PATCH + JSON** = แก้ metadata

### นอก envelope (สำหรับ probe / gateway)

| Method | Path | ใช้ทำอะไร |
|--------|------|-----------|
| GET | `/` | ชื่อ service, env, version, prefix |
| GET | `/health` · `/health/ready` | 200 เมื่อ DB ต่อได้และ migration ครบ, 503 เมื่อไม่ครบ |
| GET | `/health/live` | 200 ตราบใดที่ process ยังอยู่ |

### Panels

| Method | Path |
|--------|------|
| GET | `/panels` |
| POST | `/panels` |
| GET | `/panels/code/{code}` |
| GET · PUT · PATCH · DELETE | `/panels/{id}` |
| DELETE | `/panels/{id}/permanent` |
| POST | `/panels/{id}/restore` |
| GET · POST | `/panels/{id}/devices` |
| GET · POST | `/panels/{id}/images` |
| GET · PUT · PATCH · DELETE | `/panels/{id}/images/{imageId}` |

Filter: `active`, `has_location`, `created_from`, `created_to`, `search`  
Sort: `code`, `location`, `active`, `created_at`, `updated_at`

ทุก response มี `operational_status` (`NORMAL` / `MONITORING` / `ABNORMAL`) — **คำนวณจาก panel_devices ทุกครั้ง ไม่เก็บใน DB** (priority: MONITORING > ABNORMAL > NORMAL, ดู `business-logic-api.md` §3). ยังไม่รองรับ `?status=` filter — ทำภายหลังเมื่อจำเป็น server-side filter

**Panel images**

| Method | Content-Type | ใช้เมื่อ |
|--------|--------------|----------|
| POST | multipart | สร้างรูป (`file`, `image_type` required) |
| PUT | multipart | แทนที่ไฟล์ (ลบ S3 เก่าหลังสำเร็จ) |
| PUT | JSON | แทนที่ metadata (`image_type`, `sort_order` required) |
| PATCH | JSON | แก้ metadata บาง field |
| DELETE | — | ลบถาวร DB + S3 |

Filter images: `image_type` · Sort: `sort_order`, `created_at`, `image_type` · ทุก response มี `url`

| Method | Path |
|--------|------|
| GET · POST | `/device-models` |
| GET | `/device-models/code/{code}` |
| GET · PUT · PATCH · DELETE | `/device-models/{id}` |
| DELETE | `/device-models/{id}/permanent` |
| POST | `/device-models/{id}/restore` |

Filter: `active`, `manufacturer`, `search`

### Panel devices

| Method | Path |
|--------|------|
| GET · POST | `/panel-devices` |
| GET · PUT · PATCH · DELETE | `/panel-devices/{id}` |
| DELETE | `/panel-devices/{id}/permanent` |
| POST | `/panel-devices/{id}/restore` |
| POST | `/panel-devices/{id}/status` |
| GET · POST | `/panel-devices/{id}/calibrations` |

Filter: `panel_id`, `equipment_type`, `manufacturer`, `brand`, `active`, `communication_status`, `health_status`,
`installed_from`, `installed_to`, `last_seen_from`, `last_seen_to`, `never_seen`, `search`

ทุก response มี `operational_status` (`NORMAL` / `MONITORING` / `ABNORMAL`) — map มาจาก `communication_status` + `health_status` ในโค้ด (`internal/domain/panel_status.go`), frontend ไม่ต้องรู้ mapping rule เอง

`POST /panel-devices/{id}/status` สำหรับตัวเก็บ telemetry — ส่งเฉพาะ field ที่เปลี่ยน
ถ้าไม่ส่ง `last_seen_at` ระบบจะใช้ `now()`

### Calibration instruments

| Method | Path |
|--------|------|
| GET · POST | `/calibration-instruments` |
| GET · PUT · PATCH · DELETE | `/calibration-instruments/{id}` |
| DELETE | `/calibration-instruments/{id}/permanent` |
| POST | `/calibration-instruments/{id}/restore` |

Filter: `active`, `manufacturer`, `equipment_type`, `brand`, `expired`, `expiring_before`, `search`

Body สำคัญ: `name` (required), `equipment_type`, `manufacturer`, `brand`, `model`, `serial_number`, `calibration_date`, `expire_date`

แต่ละแถวจะมี `is_expired` และ `days_until_expiry` คำนวณมาให้

### Calibrations

| Method | Path |
|--------|------|
| GET · POST | `/calibrations` |
| GET | `/calibrations/summary` |
| GET · PUT · PATCH · DELETE | `/calibrations/{id}` |
| GET · POST · PUT | `/calibrations/{id}/readings` |
| GET · PUT · PATCH · DELETE | `/calibrations/{id}/readings/{readingId}` |
| GET · PUT · PATCH · DELETE | `/calibration-readings/{id}` |

Filter: `panel_device_id`, `panel_id`, `equipment_type`, `instrument_id`, `result`,
`performed_by`, `performed_from`, `performed_to`, `search`

**สร้างใบสอบเทียบพร้อมค่าที่วัดในครั้งเดียว** — ทั้งหมดอยู่ใน transaction เดียว
และ readings เขียนด้วย `COPY` จึงเป็น round trip เดียวแม้จะมีหลายร้อยแถว

```http
POST /api/rtu/v1/calibrations
Content-Type: application/json

{
  "panel_device_id": "…",
  "instrument_id": "…",
  "performed_by": "สมชาย",
  "performed_at": "2026-08-03T09:30:00+07:00",
  "result": "PASS",
  "remark": "ตามรอบประจำปี",
  "readings": [
    { "item_label": "Zero point",  "parameter_key": "pressure", "value": 0.02, "unit": "bar" },
    { "item_label": "Span point",  "parameter_key": "pressure", "value": 9.98, "unit": "bar" }
  ]
}
```

ถ้าไม่ส่ง `sequence` ระบบจะไล่ให้เป็น 1, 2, 3 … ตามลำดับใน array
ถ้าจะส่งเอง ต้องส่งครบทุกแถวและห้ามซ้ำ

`PUT /calibrations/{id}/readings` แทนที่ทั้งใบในครั้งเดียว (ลบของเดิม + เขียนของใหม่ ใน transaction เดียว)

สำหรับ PM 6 เดือน ส่ง `work_order_id` / `pm_report_id` ใน body ได้ (ต้องเป็น SIX_MONTH PM)

Filter เพิ่ม: `work_order_id`, `pm_report_id` (ผ่าน list query มาตรฐาน)

### Work orders (PM / CM)

`work_order_no` สร้างอัตโนมัติฝั่ง server ตอน `POST` เท่านั้น — client ส่งไม่ได้  
รูปแบบ: `{TYPE}-{panel.code}-{ลำดับ}` เช่น `PM-RTU-011-0001`, `CM-RTU-011-0042`  
ลำดับ 1–9999 เติมศูนย์ 4 หลัก; ตั้งแต่ 10000 เป็นต้นไปใช้เลขเต็ม (`10000`, `10001`, …)  
ลำดับนับแยกตาม `panel_id` + `work_order_type` (PM/CM)

| Method | Path |
|--------|------|
| GET · POST | `/work-orders` |
| GET · PUT · PATCH · DELETE | `/work-orders/{id}` |
| POST | `/work-orders/{id}/restore` |
| POST | `/work-orders/{id}/reassign` |
| GET | `/work-orders/{id}/open-cm-work-orders` |
| POST | `/work-orders/{id}/check-in` |
| POST | `/work-orders/{id}/check-out` |
| GET | `/work-orders/{id}/rounds` |
| GET | `/work-orders/{id}/activity` |
| GET · POST | `/work-orders/{id}/approvals` |
| GET · PUT | `/work-orders/{id}/pm-report` |
| POST | `/work-orders/{id}/pm-report/submit` |
| GET | `/work-orders/{id}/pm-reports` |
| GET · PUT | `/work-orders/{id}/cm-report` |
| POST | `/work-orders/{id}/cm-report/submit` |
| GET | `/work-orders/{id}/cm-reports` |
| GET · POST | `/work-orders/{id}/attachments` |

Nested ใต้ panel / device:

| Method | Path |
|--------|------|
| GET · POST | `/panels/{id}/work-orders` |
| GET | `/panels/{id}/open-cm-work-orders` |
| GET | `/panels/{id}/pm-reports` |
| GET | `/panels/{id}/cm-reports` |
| GET | `/panel-devices/{id}/work-orders` |
| GET | `/panel-devices/{id}/cm-reports` |

Filter work orders: `work_order_type`, `pm_schedule_type`, `status`, `priority`, `active`,
`assigned_to`, `panel_id`, `panel_device_id`, `planned_from/to`, `due_from/to`

Workflow สถานะ: `ASSIGNED` → `IN_PROGRESS` (check-in) → `PENDING` (check-out) →
`PENDING_APPROVAL` (submit report) → `COMPLETED` / `CONDITIONAL` / rework (reject)

### PM reports

| Method | Path |
|--------|------|
| GET · DELETE | `/pm-reports/{id}` |
| GET · POST | `/pm-reports/{id}/onsite-fixes` |
| POST | `/pm-reports/{id}/escalate` |
| GET · POST | `/pm-reports/{id}/attachments` |

`PUT /work-orders/{id}/pm-report` บันทึก aggregate ทั้งชุด (checklist + ground + power) ขณะ DRAFT
Submit บังคับ power test (PM3) หรือ calibration (PM6) ตาม `pm_schedule_type`

`GET /work-orders/{id}/pm-report` และ `GET /pm-reports/{id}` คืน field เพิ่ม `open_cm_work_orders[]`
— ใบงาน CM บนตู้เดียวกันที่ status เป็น `ASSIGNED`, `IN_PROGRESS`, `PENDING`, หรือ `PENDING_APPROVAL`
(รหัสใบงาน, สถานะ, อุปกรณ์, หัวข้อปัญหา) สำหรับแสดงการ์ดเตือนบน UI

ดึงแยกได้ที่ `GET /work-orders/{id}/open-cm-work-orders` หรือ `GET /panels/{id}/open-cm-work-orders`
Filter: `panel_device_id`, `problem_topic_id` (สำหรับจำกัดรายการ UI — duplicate check ใช้ panel + topic เท่านั้น),
`exclude_work_order_id` (ไม่นับใบงาน CM ที่กำลังแก้ไข)

### CM reports

| Method | Path |
|--------|------|
| GET · PUT · PATCH · DELETE | `/cm-reports/{id}` |
| GET · POST | `/cm-reports/{id}/attachments` |

Body สำคัญ: `problem_topic_id` (FK → `/problem-topics`, sync `tag_code` เป็น `code` อัตโนมัติ), `problem_detail`, `corrective_action`, …  
`tag_code` free text ยังรับได้ (legacy) แต่แนะนำใช้ `problem_topic_id`

### Engineers, checklist & problem master

| Method | Path |
|--------|------|
| GET · POST | `/engineers` |
| GET · PUT · PATCH · DELETE | `/engineers/{id}` |
| DELETE | `/engineers/{id}/permanent` |
| POST | `/engineers/{id}/restore` |
| GET · POST | `/checklist-items` |
| GET · PUT · PATCH · DELETE | `/checklist-items/{id}` |
| DELETE | `/checklist-items/{id}/permanent` |
| POST | `/checklist-items/{id}/restore` |
| GET · POST | `/problem-topics` |
| GET · PUT · PATCH · DELETE | `/problem-topics/{id}` |
| DELETE | `/problem-topics/{id}/permanent` |
| POST | `/problem-topics/{id}/restore` |

`GET /problem-topics?active=true` — รายการ pill หัวข้อปัญหา เรียง `sort_order` (seed: COMM_LOST, POWER_FAILURE, … OTHERS)

### Attachments (polymorphic)

| Method | Path | entity_type |
|--------|------|-------------|
| GET · POST | `/work-orders/{id}/attachments` | WORK_ORDER |
| GET · POST | `/pm-reports/{id}/attachments` | PM_REPORT |
| GET · POST | `/cm-reports/{id}/attachments` | CM_REPORT |
| GET · POST | `/calibrations/{id}/attachments` | CALIBRATION |
| GET · POST | `/panel-devices/{id}/attachments` | PANEL_DEVICE |
| GET · POST | `/pm-ground-tests/{id}/attachments` | PM_GROUND_TEST |
| GET · POST | `/pm-power-test-points/{id}/attachments` | PM_POWER_TEST_POINT |
| GET · PUT · PATCH · DELETE | `/attachments/{id}` | — |

Upload: multipart — `file` (required), `created_by` (required), `caption` (optional)

### Notifications (Screen 06)

| Method | Path |
|--------|------|
| GET · POST | `/notifications?recipient_id=` |
| GET | `/notifications/unread-count?recipient_id=` |
| POST | `/notifications/read-all?recipient_id=` |
| GET · DELETE | `/notifications/{id}?recipient_id=` |
| POST | `/notifications/{id}/read?recipient_id=` |

Type: `NEW_ASSIGNMENT`, `PENDING_WORK`, `PENDING_APPROVAL`, `COMPLETED`, `CM_PENDING`
(ระบบ emit อัตโนมัติเมื่อ assign / submit / approve / escalate)

### Panel images

| Method | Path | Content-Type | ใช้เมื่อ |
|--------|------|--------------|----------|
| GET | `/panels/{id}/images` | — | รายการรูป (มี `url` presigned) |
| POST | `/panels/{id}/images` | multipart | อัปโหลดรูปใหม่ |
| GET | `/panels/{id}/images/{imageId}` | — | ดูรายละเอียด + `url` |
| PUT | `/panels/{id}/images/{imageId}` | multipart | เปลี่ยนไฟล์รูป (ลบ S3 เก่าหลังสำเร็จ) |
| PUT | `/panels/{id}/images/{imageId}` | JSON | แทนที่ metadata ทั้งชุด |
| PATCH | `/panels/{id}/images/{imageId}` | JSON | แก้ metadata บาง field |
| DELETE | `/panels/{id}/images/{imageId}` | — | ลบถาวร (DB + S3) |

Filter: `image_type` (`EXTERIOR`, `INTERIOR`, `DEVICE`) · Sort: `sort_order`, `created_at`, `image_type`

### REST method convention

ทุก resource ที่มี `{id}` รองรับ **GET · PUT · PATCH · DELETE** ตามมาตรฐาน:

| Method | ความหมาย |
|--------|----------|
| GET | อ่าน |
| PUT | แทนที่ทั้ง resource (JSON) หรือไฟล์ (multipart สำหรับรูป) |
| PATCH | แก้บาง field |
| DELETE | ลบ |
| POST | สร้างใหม่ (ใช้ที่ collection path เท่านั้น) |

### Business rules ที่บังคับในชั้น service

* `performed_at` ล่วงหน้าเกิน 5 นาทีจากเวลาปัจจุบันไม่ได้ (`E300_121`)
* สอบเทียบอุปกรณ์ที่ `active = false` ไม่ได้ (`E300_111`)
* ใช้เครื่องมือที่ปิดใช้งาน (`E300_115`) หรือใบรับรองหมดอายุ ณ วันที่สอบเทียบ (`E300_116`) ไม่ได้
* `expire_date` ต้องหลัง `calibration_date` (`E300_117`)
* PM submit: power test บังคับสำหรับ `THREE_MONTH` (`E300_236`), calibration ≥1 สำหรับ `SIX_MONTH` (`E300_237`)
* Calibration ผูก PM ได้เฉพาะ `SIX_MONTH` work order (`E300_240`)
* Approval reject → rework เปิด round ใหม่; escalate → spawn/reuse CM work order
* Panel `last_pm_date` / `next_pm_date` sync เมื่อ PM ถึง COMPLETED หรือ CONDITIONAL
* CM report: `problem_topic_id` ต้องชี้ topic ที่ `active=true` (`E300_244`); inactive / ไม่พบ → `E300_242` / `E300_244`
* CM duplicate: ห้ามเปิด CM ใหม่ (หรือบันทึกรายงาน CM) ซ้ำบนตู้เดียวกัน + หัวข้อปัญหาเดียวกัน ขณะมีใบเปิดอยู่ (`ASSIGNED`, `IN_PROGRESS`, `PENDING`, `PENDING_APPROVAL`) → `E300_246` (panel-wide — ไม่แยก device)
* CM create: `problem_topic_id` **บังคับ** เมื่อ `work_order_type=CM` (`E300_247`) — duplicate check รัน **ใน transaction** (advisory lock ต่อ panel) ก่อน insert; seed `cm_reports` ใน tx เดียวกัน
* CM escalate (PM reject): `problem_topic_id` บังคับเมื่อ `escalate=true` — reuse CM เปิดอยู่ที่ topic ตรงกัน (`FindMatchingOpenCm`, approval path เท่านั้น)
* PM escalate (`POST .../escalate`): สร้าง CM ใหม่ผ่าน create path — ถ้ามี CM เปิด topic เดียวกันแล้ว → `E300_246`; อัปเดต seeded report ด้วย `pm_report_id`
* `PUT .../cm-report`: `problem_topic_id` **บังคับ** (`validate:"required"`) — ห้ามลบ topic ผ่าน PATCH (`E300_247`)

---

## 6. รูปแบบ request / response

### Pagination

ทุก endpoint แบบ list รับ `page`, `limit` (สูงสุด 500), `sort`, `order`, `search`

```json
{
  "status": "success",
  "timestamp": "2026-08-03T02:30:00.000Z",
  "code": "S201_001",
  "context": "LIST",
  "message": "Records fetched successfully.",
  "data": {
    "items": [],
    "meta": {
      "page": 1, "limit": 20, "total": 0, "total_pages": 0,
      "has_next": false, "has_prev": false,
      "sort": "created_at", "order": "DESC"
    }
  }
}
```

ค่า `sort` ที่ไม่อยู่ใน whitelist จะได้ `E100_004` พร้อมรายชื่อที่รับได้

### PATCH: แยก "ไม่ส่ง" ออกจาก "ส่งเป็น null"

ตัว binder อ่าน key ที่มาจริงใน JSON แล้วส่งต่อเป็น flag ไปถึง SQL

```jsonc
{ "location": "อาคาร A" }   // เปลี่ยนค่า
{ "location": null }        // ล้างเป็น NULL
{ }                         // ไม่แตะ location
```

ส่ง `null` ให้คอลัมน์ที่เป็น `NOT NULL` จะได้ `E100_003` พร้อมบอกชื่อ field
ส่ง key ที่ไม่รู้จักจะได้ `E100_002`

`PUT` กับ `PATCH` ทำงานเหมือนกัน (partial update) เพื่อให้เข้ากับ client เดิมที่ใช้ `PUT`

### Error

```json
{
  "status": "error",
  "timestamp": "2026-08-03T02:30:00.000Z",
  "code": "E300_102",
  "context": "PANEL",
  "message": "Panel code already exists.",
  "errors": [],
  "request_id": "5f0c…"
}
```

`request_id` ตรงกับ header `X-Request-Id` และกับ log บรรทัดที่เกี่ยวข้อง
รายการ code ทั้งหมดอยู่ใน [`api-response-reference.md`](./api-response-reference.md) §4

---

## 7. Operations

### Authentication

ทุก route ภายใต้ `{API_PREFIX}` ผ่าน `middleware.Auth` เมื่อ `AUTH_ENABLED=true`
(ค่าเริ่มต้น `false` ใน development เพื่อให้ smoke test ใช้งานได้ง่าย)

```http
Authorization: Bearer <access_token>
```

JWT ต้องเป็น HS256 และใช้ secret เดียวกับ MWA auth service (`AUTH_JWT_SECRET`)
Production startup จะ **ปฏิเสธ** การสตาร์ทถ้า:
- `AUTH_ENABLED=false`
- ไม่มี `AUTH_JWT_SECRET` หรือสั้นกว่า 32 ตัวอักษร
- `CORS_ALLOWED_ORIGINS=*`
- `DB_SSLMODE=disable`

### Metrics

เมื่อ `METRICS_ENABLED=true` (ค่าเริ่มต้น) มี endpoint `GET /metrics` สำหรับ Prometheus scrape
พร้อม histogram `http_request_duration_seconds` และ counter `http_requests_total` แยกตาม method/route/status

### Staging guard

เมื่อ `APP_ENV=staging` middleware `StagingGuard` จะบล็อก POST/PUT/PATCH/DELETE ทุก route
(ตอบ `423 Locked` / `E600_001`) — ป้องกันการ mutate ข้อมูลบน staging โดยไม่ตั้งใจ

### Health และ schema guard

ตอนสตาร์ต service จะเทียบ migration ที่ฝังไว้ในไบนารีกับ `public.schema_migrations`
ถ้ายังไม่ครบและ `SCHEMA_GUARD=true` (ค่าเริ่มต้น) จะไม่ยอมสตาร์ต — กันการวิ่งบน schema เก่า
ตั้ง `SCHEMA_GUARD=false` เมื่อจะ deploy โค้ดก่อน migrate โดยตั้งใจ

`/health` ตอบ 503 พร้อมรายละเอียดว่าเป็นที่ DB หรือที่ schema

### Graceful shutdown

รับ `SIGINT` / `SIGTERM` แล้วปิด listener, รอ request ที่ค้างอยู่จนถึง `HTTP_SHUTDOWN_TIMEOUT`
แล้วค่อยปิด connection pool

### หลัง PgBouncer

ถ้า proxy ทำ transaction pooling ให้ตั้ง `DB_STATEMENT_CACHE=false`
เพื่อให้ pgx เลิกใช้ server-side prepared statement

### Rate limit

Token bucket ต่อ IP (`RATE_LIMIT_REQUESTS` ต่อ `RATE_LIMIT_WINDOW`) — **in-process เท่านั้น**
เมื่อรันหลาย replica ควรปิด (`RATE_LIMIT_ENABLED=false`) แล้วให้ API gateway ทำ rate limit แทน

### Environment variables

ดู [`.env.example`](./.env.example) — ตั้งค่า `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` (หรือ override ด้วย `DATABASE_URL` ทั้งเส้น)

---

## 8. เพิ่มตารางใหม่

```bash
make migrate-create NAME=add_alarm_table   # เขียน .up.sql และ .down.sql
make migrate-up
# เขียน queries/alarms.sql
make generate                              # ได้ struct + method ที่ type-safe
```

จากนั้นเพิ่ม repository → service → handler → route ตามรูปแบบของ `panel.go` ในแต่ละชั้น
ถ้าเพิ่ม error code ใหม่ ให้เพิ่มที่ `internal/httpx/codes.go` แล้ว sync กับ `api-response-reference.md`
