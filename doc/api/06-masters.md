# 06 — Master Data

---

## Engineers

Prefix: `{api_prefix}/engineers`

| Method | Path |
|--------|------|
| `GET`/`POST` | `/engineers` |
| `GET`/`PATCH`/`DELETE` | `/engineers/{id}` |

### POST — `EngineerCreateInput`

| Field | บังคับ |
|-------|--------|
| `full_name` | ✅ |
| `employee_code` | ไม่ |
| `active` | bool |

### PATCH — `EngineerUpdateInput`

`full_name`, `employee_code`, `active` — partial

**List filter:** `active`

---

## Checklist Items

Prefix: `{api_prefix}/checklist-items`

ใช้กับ `checklist_results[].checklist_item_id` ใน PM report

**List:** `GET /checklist-items?active=true` — คืนทั้งชุด **ไม่มี pagination**

### POST — `ChecklistItemCreateInput`

| Field | บังคับ | enum |
|-------|--------|------|
| `code` | ✅ | unique |
| `name` | ✅ | |
| `action_type` | ✅ | `MAINTENANCE` / `MEASUREMENT` / `VISUAL_INSPECTION` |
| `applicable_pm` | ไม่ | `PM3` / `PM6` / `BOTH` — **default `BOTH`** ถ้าไม่ส่ง |
| `sort_order` | ✅ | ≥ 0 |
| `active` | bool | |

---

## Problem Topics

Prefix: `{api_prefix}/problem-topics`

**Frontend:** `GET /problem-topics?active=true` ก่อน create CM / save cm-report

**List:** คืนทั้งชุด **ไม่มี pagination** (filter `active` เท่านั้น)

### POST — `ProblemTopicCreateInput`

| Field | บังคับ |
|-------|--------|
| `code` | ✅ max 30 — เช่น `POWER_FAILURE` |
| `name` | ✅ |
| `sort_order` | ✅ ≥ 0 |
| `active` | bool |

### PATCH — `ProblemTopicUpdateInput`

`code`, `name`, `sort_order`, `active` — partial

**Delete:** ห้ามลบถ้ามี cm_reports อ้างอิง → `E300_245`

---

## Device Models (UI master — optional)

Prefix: `{api_prefix}/device-models`

ข้อมูล master สำหรับ UI — **panel_devices เก็บ equipment จริงบนแถวของตัวเอง** ไม่บังคับ FK

| Method | Path |
|--------|------|
| `GET`/`POST` | `/device-models` |
| `GET`/`PATCH`/`DELETE` | `/device-models/{id}` |
| `GET` | `/device-models/code/{code}` |

### POST — field สำคัญ

| Field | บังคับ |
|-------|--------|
| `code` | ✅ |
| `name` | ✅ |
| `manufacturer`, `equipment_type`, `brand` | ไม่ |
| `active` | bool |

**List filters:** `active`, `manufacturer`, `equipment_type`, `brand`
