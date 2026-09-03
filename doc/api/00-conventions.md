# 00 — Conventions (ทุก endpoint)

## Base URL

```
{base_url}{api_prefix}/...
```

ค่า default จาก config: prefix มักเป็น `/api/rtu/v1`

---

## Authentication

เมื่อ `AUTH_ENABLED=true`:

```http
Authorization: Bearer <access_token>
```

Development มักปิด auth (`AUTH_ENABLED=false`)

---

## Response envelope

### Success

```json
{
  "status": "success",
  "timestamp": "2026-09-01T06:29:21.969Z",
  "code": "S201_004",
  "context": "UPDATE",
  "message": "Record updated successfully.",
  "data": { }
}
```

| Code | HTTP | ความหมาย |
|------|------|----------|
| `S201_001` | 200 | List |
| `S201_002` | 200 | Detail |
| `S201_003` | 201 | Create |
| `S201_004` | 200 | Update |
| `S201_005` | 200 | Delete (soft) |
| `S201_007` | 200 | Restore |

### Error

```json
{
  "status": "error",
  "timestamp": "2026-09-01T06:29:21.969Z",
  "code": "E100_003",
  "context": "VALIDATION",
  "message": "Validation failed.",
  "errors": [
    {
      "field": "pm_schedule_type",
      "issue": "INVALID",
      "message": "Can only be changed while status is ASSIGNED, IN_PROGRESS, or PENDING."
    }
  ],
  "request_id": "..."
}
```

| Code | HTTP | ความหมาย |
|------|------|----------|
| `E100_002` | 400 | Unknown field (endpoint ที่ใช้ Bind แบบ strict) |
| `E100_003` | 400 | Validation failed |
| `E100_004` | 400 | sort ไม่ถูกต้อง |

---

## Pagination (GET list)

### แบบ paginated — มี `page` / `limit` / `meta`

ใช้กับ list หลัก เช่น work-orders, panels, panel-devices, calibrations, engineers, notifications, …

| Param | Default | หมายเหตุ |
|-------|---------|----------|
| `page` | 1 | |
| `limit` | 20 | สูงสุด 500 |
| `sort` | ตาม resource | ดู whitelist ในไฟล์ endpoint นั้น |
| `order` | `DESC` | `ASC` / `DESC` |
| `search` | — | ILIKE ตาม resource (ถ้า resource รองรับ) |

Response: `data.items[]` + `data.meta` (page, limit, total, has_next, …)

### แบบ full collection — ไม่มี pagination

คืน `data.items[]` ทั้งชุด ไม่มี `meta`:

| Endpoint | Filter |
|----------|--------|
| `GET /problem-topics` | `active` |
| `GET /checklist-items` | `active` |
| `GET .../attachments` (ทุก entity) | — |
| `GET /work-orders/{id}/rounds` | — |
| `GET /work-orders/{id}/activity` | — |
| `GET /work-orders/{id}/approvals` | — |
| `GET /work-orders/{id}/pm-reports` (history ใต้ใบงาน) | — |
| `GET /work-orders/{id}/cm-reports` (history ใต้ใบงาน) | — |

**หมาย:** ประวัติใต้ `/panels/{id}/pm-reports`, `/panels/{id}/cm-reports`, `/panel-devices/{id}/cm-reports` เป็น **paginated** — ดู [02-pm-reports.md](./02-pm-reports.md) / [03-cm-reports.md](./03-cm-reports.md)

---

## PATCH / PUT semantics

### PATCH (ส่วนใหญ่)

- ส่งเฉพาะ key ที่ต้องการเปลี่ยน
- **ไม่ส่ง key** = ไม่แตะ field นั้น
- **ส่ง `null`** = ล้างค่า (เฉพาะ column nullable); NOT NULL → `E100_003`
- **ส่ง key ที่ไม่รู้จัก** → `E100_002` (ยกเว้น work-order update ใช้ BindLenient — ละเว้น read-only)

### PUT

ใน API นี้ `PUT` กับ `PATCH` **ทำงานเหมือนกัน** (partial update) ยกเว้น:

| Endpoint | PUT semantics |
|----------|---------------|
| `PUT /work-orders/{id}/pm-report` | **Replace aggregate ทั้งชุด** — ทุก field ใน body มีผล (รวม null); ใช้ได้แค่ตอน report `DRAFT` |
| `PUT /work-orders/{id}/cm-report` | **Replace รายงานรอบปัจจุบัน** — ส่ง body ครบทุกครั้ง; ทุก field มีผล (รวม null = ล้าง) |
| `PUT /calibrations/{id}/readings` | **Replace ทั้ง sheet** |
| `PUT /cm-reports/{id}` | **Partial เหมือน PATCH** — ไม่ใช่ full replace |

---

## ชนิดข้อมูล

| ชนิด | รูปแบบ JSON |
|------|-------------|
| UUID | string `"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"` |
| Date | `"YYYY-MM-DD"` |
| DateTime | ISO 8601 `"2026-09-01T06:29:21.969Z"` |
| Decimal | number หรือ string ตาม field |

---

## Soft delete / restore

Resource ที่รองรับ: panels, device-models, panel-devices, engineers, checklist-items, problem-topics, calibration-instruments, work-orders

| Action | Method | Path |
|--------|--------|------|
| Soft delete | `DELETE` | `/{resource}/{id}` |
| Restore | `POST` | `/{resource}/{id}/restore` |
| Purge ถาวร | `DELETE` | `/{resource}/{id}/permanent` (บาง resource) |
