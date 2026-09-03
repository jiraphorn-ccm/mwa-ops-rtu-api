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
