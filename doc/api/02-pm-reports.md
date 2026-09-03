# 02 — PM Reports

---

## ผ่าน Work Order (flow หลัก)

### `GET /work-orders/{work_order_id}/pm-report`

คืน aggregate PM report ของ**รอบปัจจุบัน** + `open_cm_work_orders[]` (CM เปิดบนตู้เดียวกัน)

**ยังไม่เคย `PUT` สร้าง report:** → **`404`** (ไม่มี report ของ round นี้) — เริ่มด้วย `PUT` สร้าง DRAFT

### `PUT /work-orders/{work_order_id}/pm-report`

**เมื่อ:** บันทึก draft ระหว่างทำ PM (ส่งซ้ำได้)

**Semantics:** **Replace aggregate** — ทุก field ใน body มีผล (รวม `null` = ล้าง child)

**เงื่อนไข:**
- ใบงานต้องเป็น `work_order_type = PM`
- ต้องมี `current_round_id`
- รายงานต้องอยู่ในสถานะ **`DRAFT`** — หลัง submit แล้วแก้ไม่ได้ → **`409 E300_217`** (ต้องรอ reject เปิด round ใหม่)

### Request — `PmReportSaveInput`

| Field | ชนิด | หมายเหตุ |
|-------|------|----------|
| `engineer_id` | UUID | วิศวกรผู้ทำ PM |
| `note` | string | max 4000 |
| `report_date` | datetime | |
| `checklist_results` | array | ดูด้านล่าง |
| `ground_test` | object | optional |
| `power_test` | object | optional |

#### `checklist_results[]` — `ChecklistResultInput`

| Field | บังคับ |
|-------|--------|
| `checklist_item_id` | ✅ |
| `panel_device_id` | ไม่ — ต้องอยู่ใน panel |
| `status` | ไม่ |
| `value`, `meter_no`, `note` | ไม่ |
| `checked_by`, `checked_at` | ไม่ |

#### `ground_test` — `GroundTestInput`

| Field | หมายเหตุ |
|-------|----------|
| `resistance_lg`, `resistance_ng` | decimal |
| `voltage_lg`, `voltage_ng` | decimal |
| `result` | `PASS` / `FAIL` |
| `note`, `measured_by`, `measured_at` | |

#### `power_test` — `PowerTestInput`

| Field | หมายเหตุ |
|-------|----------|
| `instrument_id` | FK เครื่องมือ |
| `tested_by`, `tested_at` | |
| `points[]` | แถวอุปกรณ์ |

**`points[]` — `PowerTestPointInput`**

| Field | บังคับ / enum |
|-------|----------------|
| `equipment_role` | ✅ `CIRCUIT_BREAKER` / `DC_POWER_SUPPLY` |
| `brand`, `model` | |
| `input_accept_range`, `input_result_value`, `input_unit` | |
| `output_accept_range`, `output_result_value`, `output_unit` | |
| `result` | `ACCEPT` / `NOT_ACCEPTED` |
| `corrective_action` | |

---

### `POST /work-orders/{work_order_id}/pm-report/submit`

**เมื่อ:** ส่งรายงานเข้าอนุมัติ → WO status `PENDING_APPROVAL`

| Field | บังคับ |
|-------|--------|
| `actor_id` | ✅ |

**Validation ตาม `pm_schedule_type` บนใบงาน:**

| Schedule | บังคับก่อน submit |
|----------|-------------------|
| `THREE_MONTH` | มี `power_test` | → `E300_236` |
| `SIX_MONTH` | มี calibration ≥ 1 (ผูกใบ PM) | → `E300_237` |

---

## ผ่าน PM Report ID

### `GET /pm-reports/{id}`

Detail เต็ม + `open_cm_work_orders[]`

### `DELETE /pm-reports/{id}`

ลบได้เฉพาะขณะ **DRAFT**

---

## Onsite fix (แก้ในวัน PM)

### `POST /pm-reports/{pm_report_id}/onsite-fixes`

**เมื่อ:** แก้ปัญหาเสร็จในวัน PM — **ไม่ spawn CM work order**

| Field | บังคับ |
|-------|--------|
| `reported_by` | ✅ |
| `panel_device_id` | ไม่ |
| `problem_topic_id` | ไม่ (แนะนำส่ง) |
| `problem_detail`, `corrective_action`, … | ไม่ |
| `ended_at` | ไม่ (default now) |

Origin: `PM_ONSITE_FIX` — ไม่มี work order ของตัวเอง

---

## Escalate เป็น CM

### `POST /pm-reports/{pm_report_id}/escalate`

**เมื่อ:** แก้ onsite ไม่ได้ — spawn CM work order

**เงื่อนไข PM WO:** status ∈ `ASSIGNED`, `IN_PROGRESS`, `PENDING` → มิฉะนั้น **`409 E300_241`**

| Field | บังคับ |
|-------|--------|
| `pending_reason` | ✅ max 4000 |
| `reported_by` | ✅ |
| `assigned_to` | ✅ |
| `assigned_by` | ✅ |
| `problem_topic_id` | ✅ |
| `panel_device_id` | ไม่ |
| `tag_code` | ไม่ (legacy; แนะนำใช้ topic) |
| `error_logs`, `problem_detail` | ไม่ |
| `repair_date` | ไม่ |

**Duplicate:** ถ้ามี CM เปิด topic เดียวกันบน panel → **`409 E300_246`** (ไม่สร้างใบใหม่ — **ไม่ reuse**)

**ผล:** สร้าง CM work order ใหม่ + อัปเดต seeded `cm_report` ด้วย `pm_report_id`

### Escalate เปรียบเทียบ 2 เส้นทาง

| | `POST /pm-reports/{id}/escalate` | `POST /work-orders/{id}/approvals` (reject + escalate) |
|--|----------------------------------|--------------------------------------------------------|
| **เมื่อไหร่** | ระหว่างทำ PM หน้างาน | หลัง submit PM แล้ว reviewer ปฏิเสธ |
| **CM duplicate** | Error `409 E300_246` | **Reuse** CM ที่เปิด topic เดียวกันได้ |
| **`assigned_to`** | บังคับเสมอ | บังคับเฉพาะตอนสร้าง CM ใหม่ |
| **ดูเพิ่ม** | — | [01-work-orders.md § อนุมัติ](./01-work-orders.md#อนุมัติ) |

---

## ประวัติ

| Method | Path | Pagination |
|--------|------|------------|
| `GET` | `/work-orders/{id}/pm-reports` | ไม่ — คืนทั้งชุดของใบงานนั้น |
| `GET` | `/panels/{panel_id}/pm-reports` | **มี** — default `sort=report_date` DESC |

**Sort whitelist** (`GET /panels/{id}/pm-reports`): `report_date`, `created_at`, `round_no`

Query: `page`, `limit`, `sort`, `order` — ดู [00-conventions.md](./00-conventions.md)

---

## Attachments

| Method | Path |
|--------|------|
| `GET` / `POST` | `/pm-reports/{id}/attachments` |
| `GET` / `POST` | `/pm-ground-tests/{id}/attachments` |
| `GET` / `POST` | `/pm-power-test-points/{id}/attachments` |

ดู [07-attachments-notifications.md](./07-attachments-notifications.md)
