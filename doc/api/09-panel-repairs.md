# 09 — Panel Repairs (App API)

เอกสารนี้สำหรับ **mobile/web app** — งานซ่อมระหว่าง PM และประวัติซ่อมตู้

Prefix: `{api_prefix}/panels/{panel_id}`

อ้างอิง CM lifecycle: [01-work-orders.md](./01-work-orders.md), [03-cm-reports.md](./03-cm-reports.md)

---

## สรุป flow (อ่านก่อน implement)

ระหว่างทำ **PM** (`work_orders.status` = `IN_PROGRESS` / `ASSIGNED` / `PENDING`):

| สถานการณ์ | App เรียก | ผล |
|-----------|-----------|-----|
| พบปัญหา + **ซ่อมเสร็จหน้างาน** | `POST .../repairs/onsite` | เปิด **CM work order** + `cm_report` (งานจบแล้ว) → **ต้อง submit + approve CM แยก** |
| พบปัญหา + **ซ่อมไม่ได้** | `POST .../repairs/escalate` | เปิด **CM work order** + `cm_report` (`pending_reason`) → ทำ CM ทีหลัง |

**สำคัญ:** งานซ่อมหน้างาน **ไม่ auto-approve** — อนุมัติ PM กับอนุมัติ CM **คนละใบ**

```
PM IN_PROGRESS
  ├─ onsite  → CM WO (ASSIGNED/PENDING_APPROVAL) → POST .../cm-report/submit → POST .../approvals
  └─ escalate → CM WO (PENDING) → ทำงานภายหลัง → submit → approvals

PM ต่อด้วย PUT pm-report → submit → approvals (แยกจาก CM)
```

---

## Endpoints

| Method | Path | ใช้เมื่อ |
|--------|------|----------|
| `POST` | `/panels/{panel_id}/repairs/onsite` | ซ่อมเสร็จหน้างาน — เปิด CM |
| `POST` | `/panels/{panel_id}/repairs/escalate` | ซ่อมไม่ได้ — เปิด CM ค้าง |
| `GET` | `/panels/{panel_id}/repair-history` | ประวัติซ่อมทั้งตู้ |
| `GET` | `/panels/{panel_id}/repair-activity` | timeline audit งานซ่อม |

Legacy (ยังใช้ได้):

| Method | Path |
|--------|------|
| `POST` | `/pm-reports/{pm_report_id}/onsite-fixes` |
| `POST` | `/pm-reports/{pm_report_id}/escalate` |
| `GET` | `/panels/{panel_id}/cm-reports` |

---

## POST `/panels/{panel_id}/repairs/onsite`

เปิด CM work order สำหรับงานที่ **ทำเสร็จแล้ว** ระหว่าง PM

### Request

| Field | บังคับ | หมายเหตุ |
|-------|--------|----------|
| `pm_report_id` | ✅ | ต้อง belong ตู้ `panel_id` |
| `reported_by` | ✅ | |
| `assigned_to` | ✅ | ผู้รับ CM WO |
| `assigned_by` | ✅ | |
| `problem_topic_id` หรือ `problem_topic_ids` | ✅ | อย่างน้อย 1 topic |
| `panel_device_id` | แนะนำ | |
| `corrective_action` | แนะนำ | สิ่งที่ทำไปแล้ว |
| `problem_detail`, `root_cause`, … | ไม่ | |
| `repaired_by`, `started_at`, `ended_at` | ไม่ | `ended_at` default = now |
| `submit_for_approval` | ไม่ | `true` = server submit CM ทันที |
| `actor_id` | เงื่อนไข | ใช้ตอน submit (default = `reported_by`) |

### Response — `PmRepairOpenOutcome`

| Field | หมายเหตุ |
|-------|----------|
| `cm_report` | แถว `cm_reports` |
| `cm_work_order_id` | UUID ใบ CM |
| `cm_work_order_no` | เช่น `CM-RTU-...` |
| `cm_work_order_status` | `ASSIGNED` หรือ `PENDING_APPROVAL` ถ้า submit แล้ว |
| `origin` | `PM_ONSITE_CM` |
| `submitted_for_approval` | bool |

### ขั้นตอนหลัง onsite (ถ้าไม่ส่ง `submit_for_approval`)

```
POST /work-orders/{cm_work_order_id}/cm-report/submit
  body: { "actor_id": "..." }

POST /work-orders/{cm_work_order_id}/approvals
  body: { "reviewer_id", "decision": "APPROVED", ... }
```

### Activity

- PM work order: `ONSITE_CM_OPENED` (note มีเลข CM)
- CM work order: `ASSIGNED` → `STATUS_CHANGED` → `SUBMITTED` → `APPROVED`

---

## POST `/panels/{panel_id}/repairs/escalate`

เปิด CM สำหรับงานที่ **ยังทำไม่เสร็จ**

Body เหมือน [02-pm-reports § escalate](./02-pm-reports.md#escalate-เป็น-cm) + `pm_report_id`

| Field | บังคับ |
|-------|--------|
| `pm_report_id` | ✅ |
| `pending_reason` | ✅ |
| `reported_by`, `assigned_to`, `assigned_by`, `problem_topic_id` | ✅ |

Response: `PmRepairOpenOutcome` — `origin` = `PM_ESCALATED`, CM มักอยู่ `ASSIGNED`/`PENDING`

PM work order activity: `CM_SPAWNED`

---

## GET `/panels/{panel_id}/repair-history`

ประวัติซ่อม (paginated) — ข้อมูลจาก `cm_reports` + enrich

### Query

| Param | หมายเหตุ |
|-------|----------|
| `page`, `limit`, `sort`, `order` | default `sort=created_at` DESC |
| `completed` | `true` = CM อนุมัติแล้ว (`COMPLETED`/`CONDITIONAL`) |
| `origin` | comma-separated: `STANDALONE`, `PM_ONSITE_CM`, `PM_ESCALATED`, `PM_ONSITE_FIX_LEGACY` |
| `panel_device_id` | filter ต่ออุปกรณ์ |

### Response item — `RepairHistoryView`

| Field | หมายเหตุ |
|-------|----------|
| (fields จาก `cm_report`) | ครบ |
| `work_order_no`, `work_order_status` | ใบ CM |
| `problem_topic_code`, `problem_topic_name` | |
| `pm_work_order_no` | ใบ PM ต้นทาง (ถ้ามี) |
| `origin` | ดูตารางด้านล่าง |
| `is_completed` | `true` เมื่อ CM อนุมัติแล้ว |

| `origin` | ความหมาย |
|----------|----------|
| `STANDALONE` | แจ้ง CM ตรง |
| `PM_ONSITE_CM` | ซ่อมหน้างาน PM → CM WO |
| `PM_ESCALATED` | escalate จาก PM |
| `PM_ONSITE_FIX_LEGACY` | ข้อมูลเก่าก่อน refactor (ไม่มี CM WO) |

---

## GET `/panels/{panel_id}/repair-activity`

Timeline audit — รวม activity ของ CM WO บนตู้ + `CM_SPAWNED` / `ONSITE_CM_OPENED` บน PM WO

Query: `page`, `limit` (sort ตาม `created_at` DESC)

---

## Duplicate CM

เปิด CM ใหม่บน panel + topic เดียวกัน ขณะมี CM เปิด → **`409 E300_246`**

Open statuses: `ASSIGNED`, `IN_PROGRESS`, `PENDING`, `PENDING_APPROVAL`

---

## ตัวอย่าง sequence — ซ่อมหน้างาน + อนุมัติ

```http
POST /api/rtu/v1/panels/{panel_id}/repairs/onsite
{
  "pm_report_id": "...",
  "reported_by": "...",
  "assigned_to": "...",
  "assigned_by": "...",
  "problem_topic_id": "...",
  "panel_device_id": "...",
  "corrective_action": "เปลี่ยนสาย signal",
  "submit_for_approval": true,
  "actor_id": "..."
}
```

Response: `cm_work_order_status` = `PENDING_APPROVAL`

```http
POST /api/rtu/v1/work-orders/{cm_work_order_id}/approvals
{
  "reviewer_id": "...",
  "decision": "APPROVED"
}
```

```http
GET /api/rtu/v1/panels/{panel_id}/repair-history?completed=true
```

→ แถว onsite ปรากฏ `is_completed: true`
