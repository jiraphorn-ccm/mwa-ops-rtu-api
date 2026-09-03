# 07 — Attachments & Notifications

---

## Attachments (polymorphic)

Max file size: **10 MB**  
Storage: S3 (presigned URL ใน response)

### Entity types (`entity_type`)

| entity_type | List/Create path |
|-------------|------------------|
| `WORK_ORDER` | `/work-orders/{id}/attachments` |
| `PM_REPORT` | `/pm-reports/{id}/attachments` |
| `CM_REPORT` | `/cm-reports/{id}/attachments` |
| `CALIBRATION` | `/calibrations/{id}/attachments` |
| `PM_GROUND_TEST` | `/pm-ground-tests/{id}/attachments` |
| `PM_POWER_TEST_POINT` | `/pm-power-test-points/{id}/attachments` |
| `PANEL_DEVICE` | `/panel-devices/{id}/attachments` |

### Standalone

| Method | Path | หมายเหตุ |
|--------|------|----------|
| `GET` | `/attachments/{id}` | detail + presigned URL |
| `PATCH` / `PUT` | `/attachments/{id}` | แก้ **`caption`** เท่านั้น (ไฟล์ immutable — ลบแล้ว upload ใหม่) |
| `DELETE` | `/attachments/{id}` | ลบจาก S3 + DB |

### Upload — `POST` (multipart/form-data)

| Form field | บังคับ | หมายเหตุ |
|------------|--------|----------|
| `file` | ✅ | binary, max 10 MB |
| `created_by` | ✅ | UUID ผู้อัปโหลด (form field — ไม่ใช่ header) |
| `caption` | ไม่ | คำอธิบายไฟล์ (max 2000) |

MIME ที่รองรับ common: jpeg, png, webp, gif, pdf

**List attachments:** `GET .../attachments` คืนทั้งชุด **ไม่มี pagination**

---

## Notifications

Prefix: `{api_prefix}/notifications`

**สำคัญ:** API นี้ **ไม่ดึง recipient จาก auth** — ต้องส่ง `recipient_id` (UUID ของผู้รับ) ทุกเส้นที่อ่าน/แก้ inbox ของคนนั้น (แนวเดียวกับ `actor_id` / `created_by`)

| Method | Path | `recipient_id` |
|--------|------|----------------|
| `GET` | `/notifications` | query **บังคับ** + `page`, `limit`, `is_read`, `type` |
| `GET` | `/notifications/unread-count` | query **บังคับ** |
| `GET` | `/notifications/{id}` | ไม่ต้อง (detail ตาม id) |
| `POST` | `/notifications/{id}/read` | body JSON **บังคับ** |
| `POST` | `/notifications/read-all` | body JSON **บังคับ** |
| `DELETE` | `/notifications/{id}` | query **บังคับ** |
| `POST` | `/notifications` | ใน body เป็น `recipient_id` ของผู้รับ (สร้างโดย system/admin) |

### ตัวอย่าง

```http
GET /notifications?recipient_id=4bccb3c4-899d-439a-9423-f782f8ba4f52&page=1&limit=20
GET /notifications/unread-count?recipient_id=4bccb3c4-899d-439a-9423-f782f8ba4f52
DELETE /notifications/{id}?recipient_id=4bccb3c4-899d-439a-9423-f782f8ba4f52
```

```json
POST /notifications/{id}/read
POST /notifications/read-all
{ "recipient_id": "4bccb3c4-899d-439a-9423-f782f8ba4f52" }
```

### POST create — `NotificationCreateInput`

| Field | บังคับ |
|-------|--------|
| `recipient_id` | ✅ |
| `type` | ✅ — `NEW_ASSIGNMENT`, `PENDING_WORK`, `PENDING_APPROVAL`, `COMPLETED`, `CM_PENDING` |
| `work_order_id` | ไม่ |
| `title`, `message` | ไม่ |

**List filters:** `is_read`, `type` (ใช้คู่กับ `recipient_id` ใน query)

---

## Health (ไม่ต้อง auth prefix บางตัว)

| Method | Path |
|--------|------|
| `GET` | `/health`, `/health/live`, `/health/ready` |
