# 01 — Work Orders (PM / CM)

Prefix: `{api_prefix}/work-orders` และ nested ใต้ `/panels/{panel_id}/work-orders`

---

## สร้างใบงาน

### `POST /work-orders`

### `POST /panels/{panel_id}/work-orders`

เส้นเดียวกัน — เส้นที่สอง **ไม่ต้องส่ง `panel_id`** (server ใส่จาก URL)

**Response:** `201 S201_003` + `WorkOrderView`  
**`work_order_no`** สร้าง server เท่านั้น: `{TYPE}-{panel.code}-{seq}` เช่น `PM-RTU-00003-0001`

### Request body — `WorkOrderCreateInput`

| Field | ชนิด | บังคับ | เงื่อนไข |
|-------|------|--------|----------|
| `work_order_type` | string | ✅ | `"PM"` หรือ `"CM"` |
| `requested_by` | UUID | ✅ | ผู้เปิดใบ |
| `assigned_to` | UUID | ✅ | ผู้รับผิดชอบรอบแรก |
| `assigned_by` | UUID | ✅ | ผู้มอบหมาย |
| `panel_id` | UUID | ✅* | *ไม่ต้องส่งถ้า POST ใต้ `/panels/{id}/work-orders` |
| `pm_schedule_type` | string | เงื่อนไข | **บังคับเมื่อ PM** — `"THREE_MONTH"` หรือ `"SIX_MONTH"` exactly |
| `pm_schedule_type` | — | ห้าม | **ห้ามส่งเมื่อ CM** → `E300_206` |
| `problem_topic_id` | UUID | เงื่อนไข | **บังคับอย่างน้อย 1 หัวข้อเมื่อ CM** — ส่ง UUID เดียว หรือใช้คู่กับ `problem_topic_ids` |
| `problem_topic_ids` | UUID[] | เงื่อนไข | เปิด CM หลายหัวข้อในใบเดียว — ตรวจซ้ำ **ทีละ topic** (panel + topic + สถานะเปิด) |
| `problem_topic_id` | — | ห้าม | **ห้ามส่งเมื่อ PM** → `E300_248` |
| `panel_device_id` | UUID | ไม่ | ต้องอยู่ใน panel ที่เลือก |
| `title` | string | ไม่ | max 255 |
| `description` | string | ไม่ | max 4000 |
| `priority` | string | ไม่ | `HIGH` / `MEDIUM` / `LOW` (default MEDIUM) |
| `source` | string | ไม่ | `WORKFLOW` / `LEGACY_IMPORT` |
| `related_work_order_id` | UUID | ไม่ | ใช้เมื่อ spawn จาก PM |
| `planned_date` | date | ไม่ | `YYYY-MM-DD` |
| `due_date` | date | ไม่ | `YYYY-MM-DD` |

### CM duplicate (create)

- ห้ามเปิด CM ใหม่ถ้ามี CM **เปิดอยู่** บน panel เดียวกัน + **problem topic เดียวกัน** (panel-wide)
- Open status: `ASSIGNED`, `IN_PROGRESS`, `PENDING`, `PENDING_APPROVAL`
- Error: **`409 E300_246`** — อย่า retry loop; แสดงใบเดิมจาก `errors[].message` / `work_order_id`
- Server seed `cm_reports` แถวแรกใน transaction เดียวกับ create
- เก็บ topic ทั้งหมดใน `work_order_problem_topics`; `cm_reports.problem_topic_id` = หัวข้อหลักของรอบ (topic แรกตอนสร้าง)
- เมื่อ PUT/PATCH cm-report เปลี่ยน topic → sync junction (เพิ่ม topic ใหม่ถ้ายังไม่มี — **ไม่ลบ** topic ที่ตั้งตอน create หรือ sync ก่อนหน้า)

### ตัวอย่าง PM

```json
{
  "work_order_type": "PM",
  "pm_schedule_type": "THREE_MONTH",
  "panel_id": "9fb86a82-e73f-41e7-88cf-a83afe8e578d",
  "requested_by": "4bccb3c4-899d-439a-9423-f782f8ba4f52",
  "assigned_to": "4bccb3c4-899d-439a-9423-f782f8ba4f52",
  "assigned_by": "4bccb3c4-899d-439a-9423-f782f8ba4f52",
  "title": "PM 3 เดือน",
  "priority": "HIGH",
  "planned_date": "2026-08-31"
}
```

### ตัวอย่าง CM (topic เดียว)

```json
{
  "work_order_type": "CM",
  "panel_id": "9fb86a82-e73f-41e7-88cf-a83afe8e578d",
  "problem_topic_id": "<uuid จาก GET /problem-topics>",
  "requested_by": "...",
  "assigned_to": "...",
  "assigned_by": "..."
}
```

### ตัวอย่าง CM (หลาย topic)

```json
{
  "work_order_type": "CM",
  "panel_id": "9fb86a82-e73f-41e7-88cf-a83afe8e578d",
  "problem_topic_ids": [
    "<uuid topic 1>",
    "<uuid topic 2>"
  ],
  "requested_by": "...",
  "assigned_to": "...",
  "assigned_by": "..."
}
```

**ห้าม** ส่ง array ใน `problem_topic_id` — ใช้ `problem_topic_ids` แทน

---

## อ่าน / ค้นหา

### `GET /work-orders/{id}`

**Response:** `WorkOrderView` — รวม `panel_code`, `current_assigned_to`, `current_check_in_at`, …

### `GET /work-orders`

### `GET /panels/{panel_id}/work-orders`

### `GET /panel-devices/{device_id}/work-orders`

**Query filters:**

| Param | ค่า |
|-------|-----|
| `work_order_type` | `PM` / `CM` |
| `pm_schedule_type` | `THREE_MONTH` / `SIX_MONTH` |
| `status` | หนึ่งหรือหลายสถานะ — `?status=ASSIGNED&status=IN_PROGRESS` หรือ `?status=ASSIGNED,PENDING` หรือ `?statuses=...` |
| `priority` | `HIGH`, `MEDIUM`, `LOW` |
| `active` | boolean |
| `assigned_to` | UUID (ผู้รับผิดชอบรอบปัจจุบัน) |
| `panel_id` | UUID (เฉพาะ GET /work-orders) |
| `panel_device_id` | UUID |
| `problem_topic_id` | UUID (เฉพาะ CM — topic ที่เก็บบนใบงาน) |
| `planned_from` / `planned_to` | datetime |
| `due_from` / `due_to` | datetime |

**Pagination:** มี `page`, `limit`, `search`, `sort`, `order`

**Sort whitelist** (`sort` param, default `created_at` DESC):

| `sort` value | Column |
|--------------|--------|
| `work_order_no` | เลขใบงาน |
| `work_order_type` | PM / CM |
| `status` | สถานะ |
| `priority` | ความสำคัญ |
| `planned_date` | วันที่วางแผน |
| `due_date` | วันครบกำหนด |
| `panel_code` | รหัสตู้ |
| `created_at` | วันสร้าง (default) |
| `updated_at` | วันแก้ล่าสุด |

---

## PATCH / PUT `/work-orders/{id}`

**ใช้เมื่อ:** แก้ metadata ใบงาน (ไม่เปลี่ยนสถานะ / ไม่เปลี่ยนผู้รับผิดชอบ)

**BindLenient:** ส่ง field จาก GET ที่แก้ไม่ได้มาด้วยได้ — จะถูก**ละเว้น**  
**ถ้าส่งเฉพาะ field ที่แก้ไม่ได้** → `400 E100_003` + รายการ key ที่รับได้

### Field ที่แก้ได้ — `WorkOrderUpdateInput`

| Field | ชนิด | เงื่อนไข |
|-------|------|----------|
| `title` | string | max 255; ส่ง `null` ล้างได้ |
| `description` | string | max 4000; ส่ง `null` ล้างได้ |
| `priority` | string | `HIGH` / `MEDIUM` / `LOW`; **ถ้าส่งต้องไม่ null** |
| `planned_date` | date | `YYYY-MM-DD`; ส่ง `null` ล้างได้ |
| `due_date` | date | `YYYY-MM-DD`; ส่ง `null` ล้างได้ |
| `panel_device_id` | UUID | ต้องอยู่ใน panel ของใบ; ส่ง `null` ล้างได้ |
| `pm_schedule_type` | string | **เฉพาะ PM** + status ∈ `ASSIGNED`, `IN_PROGRESS`, `PENDING` |
| `problem_topic_id` | UUID | **เฉพาะ CM** — ส่งคู่กับหรือแทน `problem_topic_ids` (replace ชุด topic ทั้งใบ) |
| `problem_topic_ids` | UUID[] | **เฉพาะ CM** — replace รายการ topic บน junction; ตรวจซ้ำเฉพาะ topic ที่**เพิ่มใหม่** |

### `problem_topic_ids` บน PATCH (CM)

| เงื่อนไข | HTTP | หมายเหตุ |
|----------|------|----------|
| `work_order_type = PM` | 400 | `E300_248` |
| status ∉ `ASSIGNED`, `IN_PROGRESS`, `PENDING` | 409 | `E300_204` |
| ไม่มี topic อย่างน้อย 1 | 400 | `E300_247` |
| topic ใหม่ชน CM เปิดบน panel | 409 | `E300_246` |
| มี cm-report รอบปัจจุบัน แต่ list ไม่มี `problem_topic_id` ของ report | 400 | เปลี่ยน report ก่อน หรือคง topic นั้นไว้ใน list |

- Replace ทั้งชุด — topic ที่ถอดออกจะไม่ block CM ใหม่บน topic นั้น
- PUT/PATCH cm-report ยัง **add-only** topic เข้า junction (ไม่ลบ topic ที่มีอยู่)

### `pm_schedule_type` — รายละเอียด

| เงื่อนไข | HTTP | Code |
|----------|------|------|
| `work_order_type = CM` + ส่ง field | 400 | `E300_206` |
| status เป็น `PENDING_APPROVAL`, `COMPLETED`, `CONDITIONAL`, `CANCELLED` | 409 | `E300_204` |
| ค่าไม่ใช่ `THREE_MONTH` / `SIX_MONTH` | 400 | `E100_003` |
| ส่ง `null` | 400 | `E300_205` (required for PM) |

**หมายเหตุ:** เปลี่ยน schedule หลังเริ่มทำงานได้ตามที่ business เปิด — แต่ submit PM ยังใช้กฎ power test (3 เดือน) / calibration (6 เดือน) ตามค่า**ล่าสุด**บนใบ

### Field ที่แก้ไม่ได้ (ใช้ endpoint อื่น)

| Field | เหตุผล | ใช้แทน |
|-------|--------|--------|
| `panel_id` | ผูกเลขใบงาน + ลำดับ | สร้างใบใหม่ |
| `work_order_type` | PM/CM คนละ flow | สร้างใบใหม่ |
| `status` | audit workflow | check-in/out, submit, approve |
| `requested_by` | audit ผู้เปิด | — |
| `assigned_to` | อยู่ที่ round | `POST .../reassign` |
| `work_order_no` | server-generated | — |

### ตัวอย่าง PATCH ที่ถูก

```json
{
  "title": "ถถถ",
  "description": "รายละเอียด",
  "priority": "HIGH",
  "planned_date": "2026-08-31",
  "pm_schedule_type": "SIX_MONTH"
}
```

---

## Workflow actions

### `POST /work-orders/{id}/reassign`

**เมื่อ:** เปลี่ยนผู้รับผิดชอบ **ก่อน check-in** รอบปัจจุบัน

| Field | บังคับ |
|-------|--------|
| `assigned_to` | ✅ |
| `actor_id` | ✅ |

**Error:** check-in แล้ว → `409 E300_208` (ต้อง reject เปิด round ใหม่แทน)

---

### `POST /work-orders/{id}/check-in`

**เมื่อ:** ช่างเริ่มงาน → status → `IN_PROGRESS`

| Field | บังคับ |
|-------|--------|
| `check_in_at` | ไม่ (default now) |
| `lat`, `lng` | ไม่ |

---

### `POST /work-orders/{id}/check-out`

**เมื่อ:** ช่างออกจากหน้างาน — บันทึก `check_out_at` บน round ปัจจุบัน

**สถานะใบงาน:** ยัง **`IN_PROGRESS`** (ไม่เปลี่ยนเป็น `PENDING`) — เปลี่ยนเป็น `PENDING_APPROVAL` เมื่อ submit report เท่านั้น

| Field | บังคับ |
|-------|--------|
| `check_out_at` | ไม่ (default now) |
| `lat`, `lng` | ไม่ |

**Error:** ยังไม่ check-in → `409 E300_209`; check-out ซ้ำ → `409 E300_210`

---

## Open CM บนตู้เดียวกัน (เตือน UI)

### `GET /work-orders/{id}/open-cm-work-orders`

### `GET /panels/{panel_id}/open-cm-work-orders`

**Query (optional):**

| Param | ใช้ทำ |
|-------|--------|
| `panel_device_id` | กรองรายการ UI |
| `problem_topic_id` | กรองรายการ UI |
| `exclude_work_order_id` | ไม่นับใบ CM ที่กำลังแก้ |

**หมายเหตุ:** duplicate check ตอน create CM ใช้ **panel + topic** เท่านั้น (ไม่แยก device)

**Response item:** `work_order_id`, `work_order_no`, `status`, `problem_topic_id`, `problem_topic_name`, device fields, …

---

## ประวัติ / audit

| Method | Path | คืน |
|--------|------|-----|
| `GET` | `/work-orders/{id}/rounds` | รอบงานทั้งหมด |
| `GET` | `/work-orders/{id}/activity` | activity log |
| `GET` | `/work-orders/{id}/approvals` | ประวัติอนุมัติ |
| `GET` | `/work-orders/{id}/pm-reports` | ประวัติ PM report ทุกรอบ |
| `GET` | `/work-orders/{id}/cm-reports` | ประวัติ CM report ทุกรอบ |

---

## อนุมัติ

### `POST /work-orders/{id}/approvals`

**เมื่อ:** status = `PENDING_APPROVAL` เท่านั้น

| Field | ชนิด | เงื่อนไข |
|-------|------|----------|
| `reviewer_id` | UUID | ✅ |
| `decision` | string | ✅ `APPROVED` / `APPROVED_CONDITION` / `REJECTED` |
| `note` | string | ไม่ |
| `reassign_to` | UUID | REJECTED rework — ไม่ส่ง = assignee เดิม |
| `escalate` | bool | REJECTED + spawn CM |
| `repair_date` | date | **บังคับเมื่อ escalate=true** |
| `problem_topic_id` | UUID | **บังคับเมื่อ escalate=true** |
| `assigned_to` | UUID | บังคับเมื่อ escalate=true **และ** ไม่มี CM เปิด topic นี้อยู่แล้ว |

**ผล REJECTED rework:** ใบเดิม status → `PENDING`, เปิด round ใหม่ (assignee จาก `reassign_to` หรือคนเดิม)

**ผล REJECTED escalate:** ใบ PM → `CONDITIONAL` + spawn หรือ **reuse** CM ที่เปิด topic เดียวกันบน panel แล้ว (ไม่ error duplicate — ต่างจาก `POST /pm-reports/{id}/escalate`)

| Error | HTTP | เมื่อ |
|-------|------|-------|
| `E300_213` | 400 | escalate แต่ไม่ส่ง `repair_date` |
| `E300_247` | 400 | escalate แต่ไม่ส่ง `problem_topic_id` |
| `E300_214` | 400 | escalate + ไม่มี CM เปิดอยู่ แต่ไม่ส่ง `assigned_to` |

---

## ยกเลิก / คืนสถานะ

| Method | Path | ผล |
|--------|------|-----|
| `DELETE` | `/work-orders/{id}` | Soft cancel (`active=false`, status `CANCELLED`) |
| `POST` | `/work-orders/{id}/restore` | คืน active |

---

## Nested reports & attachments

| งาน | Path | ดูเอกสาร |
|-----|------|----------|
| PM report | `/work-orders/{id}/pm-report` | [02-pm-reports.md](./02-pm-reports.md) |
| CM report | `/work-orders/{id}/cm-report` | [03-cm-reports.md](./03-cm-reports.md) |
| ไฟล์แนบ | `/work-orders/{id}/attachments` | [07-attachments-notifications.md](./07-attachments-notifications.md) |

---

## WorkOrderView (GET) — field สำคัญ

| Field | ความหมาย | แก้ผ่าน PATCH? |
|-------|----------|----------------|
| `id`, `work_order_no` | อ่านอย่างเดียว | ❌ |
| `work_order_type` | PM/CM | ❌ |
| `pm_schedule_type` | THREE_MONTH/SIX_MONTH | ✅ ตามเงื่อนไขด้านบน |
| `panel_id`, `panel_code` | ตู้ | ❌ panel_id |
| `problem_topic_id` | หัวข้อแรก (backward compat) | ❌ |
| `problem_topics` | รายการ `{id, code, name}` ทุกหัวข้อของใบ CM | ❌ |
| `problem_topic_code`, `problem_topic_name` | หัวข้อแรก (backward compat) | ❌ |
| `status` | workflow | ❌ |
| `priority`, `title`, `description` | metadata | ✅ |
| `planned_date`, `due_date` | วันที่ | ✅ |
| `requested_by` | ผู้เปิด | ❌ |
| `current_assigned_to` | ผู้ทำรอบปัจจุบัน | ❌ → reassign |
| `current_check_in_at` | เวลา check-in | ❌ |
| `active` | ยังใช้งาน | ❌ → DELETE/restore |
