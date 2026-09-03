# 03 — CM Reports

---

## ผ่าน Work Order (flow หลัก)

### `GET /work-orders/{work_order_id}/cm-report`

รายงาน CM ของ**รอบปัจจุบัน** (ถ้าไม่มี → 404)

### `PUT /work-orders/{work_order_id}/cm-report`

**เมื่อ:** บันทึก/อัปเดตรายงาน CM ของรอบปัจจุบัน

**Semantics:** **Replace รายงานรอบปัจจุบัน** — ส่ง body **ครบทุกครั้ง**; ทุก field ใน body มีผล (รวม `null` = ล้างค่า) — ไม่ใช่ partial update

### Request — `CmReportSaveInput`

| Field | ชนิด | บังคับ | เงื่อนไข |
|-------|------|--------|----------|
| `problem_topic_id` | UUID | ✅ | topic ต้อง `active=true` |
| `reported_by` | UUID | ไม่ | default = `requested_by` ของใบ |
| `panel_device_id` | UUID | ไม่ | ต้องอยู่ใน panel |
| `tag_code` | string | ไม่ | legacy; sync จาก topic `code` ถ้ามี topic |
| `error_logs` | text | ไม่ | |
| `problem_detail` | text | ไม่ | |
| `root_cause` | text | ไม่ | |
| `reference_info` | text | ไม่ | |
| `corrective_action` | text | ไม่ | |
| `recommendation` | text | ไม่ | |
| `pending_reason` | text | ไม่ | |
| `repaired_by` | UUID | ไม่ | |
| `reported_at`, `started_at`, `ended_at` | datetime | ไม่ | |

**Duplicate:** เปลี่ยน topic เป็น topic ที่มี CM เปิดอยู่บน panel แล้ว → **`409 E300_246`** (ยกเว้นใบตัวเอง)

---

### `POST /work-orders/{work_order_id}/cm-report/submit`

| Field | บังคับ |
|-------|--------|
| `actor_id` | ✅ |

→ status ใบงาน `PENDING_APPROVAL`

---

## แก้รายงานโดยตรง

### `GET /cm-reports/{id}`

### `PUT /cm-reports/{id}`

### `PATCH /cm-reports/{id}`

**Semantics:** PUT และ PATCH **ทำงานเหมือนกัน** — partial update (ส่งเฉพาะ field ที่จะเปลี่ยน)

- Bind strict — key ที่ไม่รู้จัก → `400 E100_002`
- **ไม่ใช่** full replace (ต่างจาก `PUT /work-orders/{id}/cm-report`)

`CmReportUpdateInput` — field เดียวกับ Save แต่ optional ทุกตัว

| Field | เงื่อนไขพิเศษ |
|-------|----------------|
| `problem_topic_id` | **ห้าม PATCH เป็น null** → `E300_247` |
| `problem_topic_id` | เปลี่ยน topic → duplicate check panel+topic |

### `DELETE /cm-reports/{id}`

**ลบถาวร** — ไม่มี soft delete, ไม่จำกัดสถานะ work order (ต่างจาก PM report ที่ลบได้แค่ `DRAFT`)

ใช้ด้วยความระมัดระวัง — มักไม่จำเป็นถ้ามี work order ผู้อยู่

---

## CM Create — สรุปกฎ (POST work-orders)

| กฎ | Error |
|-----|-------|
| `problem_topic_id` บังคับ | `E300_247` |
| ห้ามส่ง topic เมื่อ PM | `E300_248` |
| ซ้ำ panel + topic ขณะ CM เปิด | `E300_246` |
| topic inactive / ไม่พบ | `E300_244` / `E300_242` |

**Open CM statuses สำหรับ duplicate:** `ASSIGNED`, `IN_PROGRESS`, `PENDING`, `PENDING_APPROVAL`

---

## Problem topics (master)

### `GET /problem-topics?active=true`

ใช้เติม pill UI ก่อน create CM / save report

| Field response | หมายเหตุ |
|----------------|----------|
| `id` | ส่งเป็น `problem_topic_id` |
| `code` | เช่น `POWER_FAILURE` — sync เป็น `tag_code` |
| `name` | แสดง UI |
| `sort_order` | เรียง pill |

---

## ประวัติ

| Method | Path | Pagination |
|--------|------|------------|
| `GET` | `/work-orders/{id}/cm-reports` | ไม่ — คืนทั้งชุดของใบงานนั้น |
| `GET` | `/panels/{panel_id}/cm-reports` | **มี** — default `sort=created_at` DESC |
| `GET` | `/panel-devices/{device_id}/cm-reports` | **มี** — default `sort=created_at` DESC |
| `GET` | `/pm-reports/{pm_report_id}/onsite-fixes` | ไม่ — คืนทั้งชุด |

**Sort whitelist** (panel / device history): `reported_at`, `started_at`, `ended_at`, `created_at`, `round_no`

Query: `page`, `limit`, `sort`, `order`

---

## Attachments

| Method | Path |
|--------|------|
| `GET` / `POST` | `/cm-reports/{id}/attachments` |
