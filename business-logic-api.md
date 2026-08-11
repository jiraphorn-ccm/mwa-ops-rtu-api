# RTU API — Business Logic

เอกสารนี้อธิบาย **กฎธุรกิจที่ API บังคับจริง** จากชั้น `handler → service → repository → PostgreSQL`  
Source of truth: `internal/service/`, `internal/handler/`, `internal/repository/`, `migrations/`

> รูปแบบ response / error code ดู [`api-response-reference.md`](./api-response-reference.md)  
> รายการ endpoint ดู [`README.md`](./README.md) §5

---

## Scrutinize Summary

**เป้าหมาย:** บันทึก business logic ให้ตรงกับโค้ดที่รันอยู่ ไม่ใช่ความตั้งใจจาก DBML

**Verdict: fix-then-ship** — logic หลักสอดคล้อง schema v2; panel images API เรียบง่าย (upload / replace / delete)

| Severity | Finding | Status |
|----------|---------|--------|
| **Major** | PATCH `calibrations/{id}` เปลี่ยนแค่ `performed_at` ไม่ re-check instrument | **Fixed** |
| **Major** | Panel images มี soft delete + device FK ซับซ้อนเกินไป | **Fixed** — hard delete + panel-only |
| **Major** | `E300_103` (Panel inactive) ไม่เคยใช้ | Open |
| **Major** | Query date filter ใช้ midnight UTC | Open — documented |

---

## 0. Audit trail (`created_by` / `updated_by`)

Migration `000002` เพิ่มคอลัมน์, `000004` เปลี่ยนเป็น **`uuid`** (MWA user id):

| Table | `created_by` | `updated_by` |
|-------|--------------|--------------|
| panels | ✓ | ✓ |
| device_models | ✓ | ✓ |
| panel_devices | ✓ | ✓ |
| calibration_instruments | ✓ | ✓ |
| calibrations | ✓ | ✓ |
| calibration_readings | ✓ | ✓ |
| panel_images | ✓ | ✓ |

**กฎการเติมค่า**

- ค่ามาจาก JWT `user_id` (fallback `sub`) — ต้องเป็น UUID ที่ parse ได้ — `repository/audit.go`
- **Create:** ตั้ง `created_by` และ `updated_by` เป็นค่าเดียวกัน
- **Update / soft delete / restore / status:** ตั้ง `updated_by` ทุกครั้ง
- **`AUTH_ENABLED=false`** หรือไม่มี token → คอลัมน์เป็น `null` (nullable)
- ชนิด: `varchar(100)` — ไม่ FK ไป user table (MWA auth เป็น service แยก)
- Response JSON มี `created_by`, `updated_by` ใน `data` อัตโนมัติ (sqlc model)

**รัน migration**

```powershell
migrate -path migrations -database "postgres://postgres:PASSWORD@127.0.0.1:5432/mwa_dev?sslmode=disable&search_path=public" up
```

---

## 1. โมเดลโดเมน

```
panels ──< panel_devices >── device_models
                │
                └──< calibrations >── calibration_instruments
                          │
                          └──< calibration_readings  (ON DELETE CASCADE)
```

| Entity | Soft delete (`active`) | Hard delete |
|--------|------------------------|-------------|
| panels | ✓ `DELETE` → `active=false` | ✓ `/permanent` — ห้ามถ้ามี devices |
| device_models | ✓ | ✓ — ห้ามถ้ามี devices ใช้รุ่นนี้ |
| panel_devices | ✓ | ✓ — ห้ามถ้ามี calibrations |
| calibration_instruments | ✓ | ✓ — ห้ามถ้ามี calibrations อ้างอิง |
| calibrations | ✗ ไม่มี `active` | ✓ `DELETE` ลบจริง + cascade readings |
| calibration_readings | ✗ | ✓ ลบทีละแถว |

---

## 2. กฎข้าม entity (Cross-cutting)

### 2.1 PATCH / PUT — partial update

- Field **ไม่ส่ง** → ไม่แก้คอลัมน์
- Field ส่ง **`null`** → ล้างคอลัมน์ nullable เป็น `NULL`
- Field ส่งค่า → อัปเดต
- Key ที่ไม่รู้จัก → `E100_002`
- ส่ง `null` ให้คอลัมน์ **NOT NULL** → `E100_003` (ผ่าน `patchRequired` ใน `patch.go`)

`PUT` กับ `PATCH` ทำงานเหมือนกัน

### 2.2 Soft delete / restore

- `DELETE /{resource}/{id}` → `active = false`, ตอบ `{ "id", "soft_deleted": true }`
- `POST /{resource}/{id}/restore` → `active = true`
- Record ที่ `active=false` **ยัง query ได้** (list กรองด้วย `?active=false`)

### 2.3 Authentication

- Route ใต้ `{API_PREFIX}` ต้อง JWT Bearer เมื่อ `AUTH_ENABLED=true`
- Algorithm: **HS256**, secret = `AUTH_JWT_SECRET`

### 2.4 Staging guard

- `APP_ENV=staging` → POST/PUT/PATCH/DELETE ถูกบล็อก → `E600_001` (HTTP 423)
- GET ยังใช้ได้

### 2.5 เวลา (wire format)

| ชนิด | รูปแบบ | หมายเหตุ |
|------|--------|----------|
| Envelope `timestamp` | UTC `...Z` | `httpx/response.go` |
| `timestamptz` fields | RFC 3339 (มักเป็น UTC) | `created_at`, `performed_at`, `last_seen_at` |
| `DATE` fields | `YYYY-MM-DD` | `installed_at`, `calibration_date`, `expire_date` |

Client แปลงเป็นเวลาไทย (`Asia/Bangkok`) ตอนแสดงผล — API **ไม่** lock เป็น +07

### 2.6 Pagination & list

- `page` ≥ 1, `limit` 1–500 (default 20)
- `sort` ต้องอยู่ใน whitelist ต่อ resource → `E100_004`
- `order`: `ASC` | `DESC` (default `DESC`)
- `search`: ILIKE ตาม field ที่กำหนดต่อ resource

---

## 3. Panels

**Base:** `/api/rtu/v1/panels`

### Create (`POST /panels`)

| Field | Required | Rule |
|-------|----------|------|
| `code` | ✓ | max 20, **unique** → `E300_102` |
| `location` | | max 4000, nullable |
| `latitude` | | -90 … 90 → validation / DB CHECK |
| `longitude` | | -180 … 180 |
| `active` | | default `true` (DB) |

### Update

- `code`, `active` เป็น NOT NULL — ส่ง `null` ไม่ได้
- พิกัด validate ช่วงก่อนเขียน DB

### Delete

| Action | Rule |
|--------|------|
| Soft delete | ตั้ง `active=false` — **ไม่เช็ค** ว่ามี devices |
| Hard delete | ห้ามถ้ามี `panel_devices` → `E300_104` |

### List filters

`active`, `has_location`, `created_from`, `created_to`, `search` (code, location)

### Computed `operational_status` (API only — ไม่เก็บใน DB)

ทุก response ของ panel (list, detail, create, update, restore) มี field `operational_status`:

| Value | ความหมาย |
|-------|----------|
| `NORMAL` | อุปกรณ์ active ทุกตัวในตู้ healthy + online |
| `MONITORING` | มีอย่างน้อย 1 อุปกรณ์ warning/degraded/unknown |
| `ABNORMAL` | มี critical/offline แต่ไม่มี monitoring |

**Priority รวมตู้:** `MONITORING` > `ABNORMAL` > `NORMAL`  
ตู้ไม่มีอุปกรณ์ active → `NORMAL`

คำนวณจาก `panel_devices.operational_status` ของอุปกรณ์ที่ `active=true` เท่านั้น (ดู §5)
— ไม่ filter ด้วย `?status=` บน endpoint นี้ตอนนี้ (ยังไม่ implement — ดู TODO §5)

---

## 4. Device Models

**Base:** `/api/rtu/v1/device-models`

### Create

| Field | Required | Rule |
|-------|----------|------|
| `code` | ✓ | max 30, unique → `E300_106` |
| `name` | ✓ | max 100 |
| `manufacturer`, `model` | | max 100 |
| `description` | | max 4000 |

### Delete

| Action | Rule |
|--------|------|
| Hard delete | ห้ามถ้ามี panel_devices ใช้รุ่นนี้ → `E300_107` |

---

## 5. Panel Devices

**Base:** `/api/rtu/v1/panel-devices`  
**Nested:** `/panels/{panel_id}/devices`

### Create

| Field | Required | Rule |
|-------|----------|------|
| `panel_id` | ✓ (flat route) | จาก body หรือ URL nested; URL ชนะ — mismatch → validation error |
| `device_model_id` | ✓ | FK ต้องมีอยู่ |
| `tag_name` | | max 100; **unique ภายใน panel** (ถ้าไม่ null) → `E300_110` |
| `serial_number` | | max 100; **unique ทั้งระบบ** → `E300_109` |
| `communication_status` | | `ONLINE` \| `OFFLINE` \| `DEGRADED` \| `UNKNOWN` (default `UNKNOWN`) |
| `health_status` | | `NORMAL` \| `WARNING` \| `CRITICAL` \| `UNKNOWN` (default `UNKNOWN`) |
| `installed_at` | | `YYYY-MM-DD` |

**ไม่เช็ค** ว่า panel หรือ device_model ยัง `active=true` ตอนสร้าง (FK อย่างเดียว)

### Computed `operational_status` (API only — ไม่เก็บใน DB)

ทุก response ของ panel device มี field `operational_status` — คำนวณจาก `communication_status` + `health_status` ในโค้ด (ไม่ใช่ SQL) เพื่อให้ frontend ไม่ต้องรู้ business rule การ mapping:

| communication_status | health_status | → `operational_status` |
|-----------------------|----------------|--------------------------|
| `OFFLINE` | ใดก็ได้ | `ABNORMAL` |
| ใดก็ได้ | `CRITICAL` | `ABNORMAL` |
| `DEGRADED` หรือ `UNKNOWN` | ใดก็ได้ (ไม่เข้าเงื่อนไข ABNORMAL) | `MONITORING` |
| ใดก็ได้ (ไม่เข้าเงื่อนไข ABNORMAL) | `WARNING` หรือ `UNKNOWN` | `MONITORING` |
| `ONLINE` | `NORMAL` | `NORMAL` |

Logic: `internal/domain/panel_status.go` (`DeviceOperationalStatus`) — ตรงกับสูตรรวมสถานะตู้ใน §3

### Record status (`POST /panel-devices/{id}/status`)

- อัปเดต telemetry: `communication_status`, `health_status`, `last_seen_at`
- **`last_seen_at` ไม่ส่ง** → DB ใช้ `now()` (`queries/panel_devices.sql`)
- Field ที่ไม่ส่ง → ไม่เปลี่ยน

### Delete

| Action | Rule |
|--------|------|
| Hard delete | ห้ามถ้ามี calibrations → `E300_112` |

### List view (computed)

- `calibration_count`, `last_calibrated_at`, `last_calibration_result` จาก calibration ล่าสุด
- Join: `panel_code`, `device_model_code`, ฯลฯ

---

## 6. Calibration Instruments

**Base:** `/api/rtu/v1/calibration-instruments`

### Certificate dates

```
expire_date > calibration_date   (ถ้าส่งทั้งคู่)
```

- Create / Update (เมื่อแก้ date) → `E300_117` ถ้าผิดลำดับ
- Update เปรียบเทียบกับค่าใน DB สำหรับ field ที่ไม่ได้ส่ง (`calibration_instrument.go:106-122`)

### List (computed)

- `is_expired`: `expire_date < current_date` (ตาม **วันที่ของ DB server**)
- `days_until_expiry`: `expire_date - current_date`
- Filter: `expired=true/false`, `expiring_before`

### Delete

| Action | Rule |
|--------|------|
| Hard delete | ห้ามถ้ามี calibrations → `E300_118` |

---

## 7. Calibrations (core business)

**Base:** `/api/rtu/v1/calibrations`  
**Nested:** `/panel-devices/{id}/calibrations`

### Create — กฎที่บังคับก่อนบันทึก

```
1. performed_at  ≤ now() + 5 minutes     → E300_121
2. panel_device.active = true              → E300_111
3. instrument.active = true                → E300_115
4. instrument certificate valid at performed_at → E300_116
5. result ∈ { PASS, FAIL, ADJUSTED }       → E300_120 / validation
6. readings ≤ 500 rows                     → E100_003
7. readings sequence: all-or-nothing, unique → validation / E300_123
```

### Certificate validity ณ `performed_at`

```go
// calibration.go:335-343
validUntil = expire_date + 1 day   // ใบรับรองใช้ได้ทั้งวันหมดอายุ
performed_at ต้อง < validUntil (UTC)
```

ตัวอย่าง: `expire_date = 2027-01-10` → สอบเทียบได้จนถึง `2027-01-10 23:59:59 UTC`

### Readings (measurement sheet)

| Rule | Detail |
|------|--------|
| Inline create | `POST /calibrations` ส่ง `readings[]` — **transaction เดียว** |
| Auto sequence | ไม่ส่ง `sequence` ทุกแถว → 1, 2, 3, … ตามลำดับ array |
| Manual sequence | ส่งครบทุกแถว ห้ามซ้ำ |
| Replace sheet | `PUT /calibrations/{id}/readings` — ลบของเดิม + เขียนใหม่ (transaction) |
| Append | `POST /calibrations/{id}/readings` — auto sequence ถ้าไม่ส่ง |
| Update calibration | **ไม่แก้ readings** — ใช้ sub-routes |

### Update calibration (`PATCH /calibrations/{id}`)

| Field changed | Re-validated |
|---------------|--------------|
| `performed_at` | Future check (`E300_121`) + instrument cert ณ วันใหม่ |
| `panel_device_id` | Device active (`E300_111`) |
| `instrument_id` | Instrument active + cert at effective `performed_at` |
| `performed_at` or `instrument_id` | รวม effective values แล้วเรียก `checkInstrumentUsable` |

### Delete

- `DELETE /calibrations/{id}` — hard delete, readings cascade (DB FK)

### Summary (`GET /calibrations/summary`)

- นับ `{ PASS, FAIL, ADJUSTED }` ตาม filter
- `performed_from` > `performed_to` → `E300_124`

---

## 8. Calibration Readings

**Base:** `/api/rtu/v1/calibration-readings/{id}`

| Action | Rule |
|--------|------|
| Get / Update / Delete | ทำงานบนแถวเดียว |
| `sequence` | ≥ 1, unique ภายใน calibration → `E300_123` |
| `parameter_key` | required ตอน create; PATCH ใช้ `patchRequired` |

---

## 9. Nested route binding

| Route | URL wins | Body mismatch |
|-------|----------|---------------|
| `POST /panels/{id}/devices` | `panel_id` | `E100_003` invalid |
| `POST /panel-devices/{id}/calibrations` | `panel_device_id` | `E100_003` invalid |

---

## 10. Business error codes (RTU)

| Code | เมื่อไหร่ |
|------|-----------|
| `E300_101` | Panel not found |
| `E300_102` | Panel code duplicate |
| `E300_103` | *(defined, unused)* Panel inactive |
| `E300_104` | Panel hard delete while devices exist |
| `E300_105` | Device model not found |
| `E300_106` | Device model code duplicate |
| `E300_107` | Device model in use |
| `E300_108` | Panel device not found |
| `E300_109` | Serial number duplicate |
| `E300_110` | Tag duplicate in panel |
| `E300_111` | Calibrate inactive device |
| `E300_112` | Device hard delete while calibrations exist |
| `E300_113` | Instrument not found |
| `E300_114` | Instrument serial duplicate |
| `E300_115` | Instrument inactive |
| `E300_116` | Instrument cert expired at `performed_at` |
| `E300_117` | `expire_date` ≤ `calibration_date` |
| `E300_118` | Instrument hard delete while in use |
| `E300_119` | Calibration not found |
| `E300_120` | Invalid `result` enum |
| `E300_121` | `performed_at` in future (>5 min skew) |
| `E300_122` | Reading not found |
| `E300_123` | Duplicate reading sequence |
| `E300_124` | Invalid date range (from > to) |

---

## 11. Flow diagrams

### สร้างใบสอบเทียบ (happy path)

```mermaid
sequenceDiagram
    participant C as Client
    participant H as Handler
    participant S as CalibrationService
    participant DB as PostgreSQL

    C->>H: POST /calibrations + readings[]
    H->>H: panel_device_id required
    H->>S: Create()
    S->>S: checkPerformedAt
    S->>S: checkDeviceUsable (active?)
    S->>S: checkInstrumentUsable (active + cert?)
    S->>S: buildReadingRows (seq rules)
    S->>DB: BEGIN → insert calibration + COPY readings
    DB-->>S: commit
    S-->>H: CalibrationDetail
    H-->>C: 201 S201_003
```

### Soft delete vs hard delete

```mermaid
flowchart TD
    A[DELETE /resource/id] --> B[active = false]
    C[DELETE /resource/id/permanent] --> D{FK references?}
    D -->|yes| E[E300_10x Conflict]
    D -->|no| F[Row deleted]
    G[calibrations DELETE] --> F
```

---

## 12. สิ่งที่ DB บังคับแต่ service ไม่ duplicate

| Constraint | DB | Service layer |
|------------|-----|---------------|
| Panel code unique | ✓ | ✓ → E300_102 |
| Lat/long range | CHECK | ✓ pre-check |
| Status enums | CHECK | ✓ validate tags |
| FK on delete | RESTRICT/CASCADE | ✓ mapped to E300_10x |
| Reading sequence unique | UNIQUE | ✓ + E300_123 |

---

## 13. Panel images (`/panels/{id}/images`)

ไฟล์จริงอยู่ **S3** — metadata อยู่ `rtu.panel_images` (อ้างแค่ `panel_id`)

### Endpoints

| Method | Path | ใช้เมื่อ |
|--------|------|----------|
| `GET` | `/panels/{id}/images` | ดูรายการรูป (ทุกรายการมี `url` presigned) |
| `POST` | `/panels/{id}/images` | อัปโหลดรูปใหม่ |
| `GET` | `/panels/{id}/images/{imageId}` | ดูรายละเอียด + `url` |
| `PUT` | `/panels/{id}/images/{imageId}` | **เปลี่ยนไฟล์รูป** (upload ใหม่ → อัปเดต DB → ลบไฟล์เก่าใน S3) |
| `PATCH` | `/panels/{id}/images/{imageId}` | แก้ metadata (`caption`, `sort_order`, `image_type`) |
| `DELETE` | `/panels/{id}/images/{imageId}` | **ลบถาวร** (DB + S3) |

### Upload / Replace (multipart/form-data)

| Field | POST | PUT (multipart) | หมายเหตุ |
|-------|------|-----------------|----------|
| `file` | required | required | jpeg, png, webp, gif — สูงสุด 10 MB |
| `image_type` | required | optional | `EXTERIOR` \| `INTERIOR` \| `DEVICE` |
| `caption` | optional | optional | |
| `sort_order` | optional | optional | |

### Metadata (application/json)

| Field | PUT | PATCH |
|-------|-----|-------|
| `image_type` | required | optional |
| `sort_order` | required | optional |
| `caption` | optional (null = ล้าง) | optional |

### Business rules

- ต้องมี S3 config (`S3_BUCKET`, credentials) — ไม่มีแล้วได้ `E300_129`
- `image_type=DEVICE` หมายถึงรูปสิ่งของในตู้ — **ไม่ FK** ไป `panel_devices`
- List แสดงเฉพาะ `active = true` (hard delete ลบแถวออกจริง)
- Replace: อัปโหลดไฟล์ใหม่ → อัปเดต record → ลบ object เก่าใน S3
- PUT + JSON = full metadata replace; PATCH = partial update

### S3 key

```
{S3_APP_PREFIX}/images/rtu/panels/{panel_code}/{uuid}.{ext}
```

---

*Generated from codebase scrutinize — อัปเดตเมื่อแก้ `internal/service/` หรือ migrations*
