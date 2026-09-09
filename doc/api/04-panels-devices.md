# 04 — Panels & Panel Devices

---

## Panels

Prefix: `{api_prefix}/panels`

| Method | Path | ใช้เมื่อ |
|--------|------|----------|
| `GET` | `/panels` | List |
| `POST` | `/panels` | สร้างตู้ |
| `GET` | `/panels/{id}` | Detail |
| `GET` | `/panels/code/{code}` | ค้นจากรหัส RTU |
| `PATCH`/`PUT` | `/panels/{id}` | แก้ข้อมูลตู้ |
| `DELETE` | `/panels/{id}` | Soft delete |
| `POST` | `/panels/{id}/restore` | Restore |
| `DELETE` | `/panels/{id}/permanent` | ลบถาวร |

### POST — `PanelCreateInput`

| Field | บังคับ | เงื่อนไข |
|-------|--------|----------|
| `code` | ✅ | max 20, unique |
| `location` | ไม่ | max 4000 |
| `latitude` | ไม่ | -90..90 |
| `longitude` | ไม่ | -180..180 |
| `install_date` | date | |
| `active` | bool | default true |

### PATCH — `PanelUpdateInput`

ทุก field optional; ส่ง `null` ล้าง nullable column ได้

**List filters:** `active`, `has_location`, `created_from`, `created_to`

---

## Panel Devices

Prefix: `{api_prefix}/panel-devices` และ `/panels/{panel_id}/devices`

| Method | Path |
|--------|------|
| `GET`/`POST` | `/panel-devices` |
| `GET`/`POST` | `/panels/{panel_id}/devices` |
| `GET`/`PATCH`/`DELETE` | `/panel-devices/{id}` |
| `POST` | `/panel-devices/{id}/status` — telemetry |
| `POST` | `/panel-devices/{id}/restore` |

### POST — `PanelDeviceCreateInput`

| Field | บังคับ | enum / หมายเหตุ |
|-------|--------|------------------|
| `panel_id` | ✅* | *จาก URL ถ้า POST ใต้ panel |
| `name` | ✅ | max 100 |
| `equipment_type`, `manufacturer`, `brand`, `model` | ไม่ | |
| `serial_number`, `tag_name`, `asset_code` | ไม่ | |
| `calibration_date`, `expire_date`, `installed_at` | date | |
| `communication_status` | ไม่ | `ONLINE`,`OFFLINE`,`DEGRADED`,`UNKNOWN` |
| `health_status` | ไม่ | `NORMAL`,`WARNING`,`CRITICAL`,`UNKNOWN` |
| `firmware_version`, `note` | ไม่ | |
| `active` | bool | |

### POST `/panel-devices/{id}/status` — telemetry

| Field | enum |
|-------|------|
| `communication_status` | ONLINE/OFFLINE/DEGRADED/UNKNOWN |
| `health_status` | NORMAL/WARNING/CRITICAL/UNKNOWN |
| `last_seen_at` | datetime |

### CM workflow — สถานะอุปกรณ์อัปเดตอัตโนมัติ

เมื่อ CM ผูก `panel_device_id` (เปิดใบงาน / บันทึก cm-report / escalate จาก PM) backend ตั้ง `health_status = WARNING` → `operational_status = MONITORING` (ติดตาม)

เมื่อ CM อนุมัติปิดงาน (`POST .../approvals`):

| ผลใบงาน | สถานะอุปกรณ์ (ถ้าไม่มี CM เปิดค้างอุปกรณ์เดียวกัน) |
|---------|-----------------------------------------------------|
| `APPROVED` → `COMPLETED` | `health_status=NORMAL` → `operational_status` ตาม telemetry (`communication_status` ไม่ถูก overwrite) |
| `APPROVED_CONDITION` → `CONDITIONAL` | `health_status=WARNING` → `operational_status=MONITORING` |

อุปกรณ์ resolve จาก `COALESCE(cm_report.panel_device_id, work_order.panel_device_id)` — client ไม่ต้องส่งเพิ่ม

ถ้ายังมี CM เปิดอยู่บนอุปกรณ์เดียวกัน จะคง `MONITORING` จนกว่า CM สุดท้ายจะปิด

ยกเลิก CM / recalc: ล้างเฉพาะ `health_status=WARNING` ที่ CM ตั้งไว้ → `NORMAL` (ไม่แตะ `CRITICAL` หรือ `communication_status`)

Sync หลัก (เปิด CM, บันทึก cm-report, อนุมัติปิด CM, ยกเลิก/คืนใบงาน) อยู่ใน **transaction เดียวกับ workflow** — ล้มเหลวจะ rollback ทั้งใบงานและสถานะอุปกรณ์

PATCH เปลี่ยน `panel_device_id` บนใบงาน CM ยัง sync แบบ best-effort หลัง commit (log เมื่อล้มเหลว)

**List filters:** `panel_id`, `equipment_type`, `manufacturer`, `brand`, `active`, status fields, date ranges, `never_seen`

---

## Panel Images

| Method | Path |
|--------|------|
| `GET`/`POST` | `/panels/{panel_id}/images` |
| `GET`/`PATCH`/`DELETE` | `/panels/{panel_id}/images/{imageId}` |

**List filter:** `image_type` = `EXTERIOR` / `INTERIOR` / `DEVICE`

---

## Nested ใต้ Panel

| Method | Path | ดูเอกสาร |
|--------|------|----------|
| `GET`/`POST` | `/panels/{id}/work-orders` | [01-work-orders.md](./01-work-orders.md) |
| `GET` | `/panels/{id}/open-cm-work-orders` | [01-work-orders.md](./01-work-orders.md) |
| `GET` | `/panels/{id}/pm-reports` | [02-pm-reports.md § ประวัติ](./02-pm-reports.md#ประวัติ) — **paginated** |
| `GET` | `/panels/{id}/cm-reports` | [03-cm-reports.md § ประวัติ](./03-cm-reports.md#ประวัติ) — **paginated** |
