# MWA API — Response Reference

เอกสารนี้อธิบายรูปแบบ response ที่ backend ใช้ตอบกลับ **ทุกกรณี** (JSON envelope, HTTP status, error/success codes)

> Base path ผ่าน Gateway: `{API_PREFIX}` (production/staging: `/api/mwa/v1`)  
> Source of truth: `shared/utils/sendSuccess.js`, `shared/utils/sendError.js`, `shared/constants/errorCodes.js`, `shared/constants/successCodes.js`

---

## 1. โครงสร้างมาตรฐาน (JSON API)

### 1.1 Success

```json
{
  "status": "success",
  "timestamp": "2026-07-31T08:00:00.000Z",
  "code": "S201_003",
  "context": "CREATE",
  "message": "Record created successfully.",
  "data": { }
}
```

| Field | Type | คำอธิบาย |
|-------|------|----------|
| `status` | `"success"` | คงที่ |
| `timestamp` | ISO 8601 UTC | เวลาตอบกลับ |
| `code` | string | รหัส success (`S200_*`, `S201_*`) |
| `context` | string | กลุ่ม use case |
| `message` | string | ข้อความภาษาอังกฤษ |
| `data` | object \| array \| null | payload จริง |

**HTTP status ที่ใช้กับ success**

| Status | ใช้เมื่อ |
|--------|----------|
| `200` | default — GET, PUT, PATCH, POST ทั่วไป |
| `201` | สร้าง record ใหม่ (create, register, upload, publish history) |

---

### 1.2 Error

```json
{
  "status": "error",
  "timestamp": "2026-07-31T08:00:00.000Z",
  "code": "E100_003",
  "context": "VALIDATION",
  "message": "Validation failed.",
  "errors": [
    {
      "field": "email",
      "issue": "INVALID",
      "message": "Invalid email"
    }
  ]
}
```

| Field | Type | คำอธิบาย |
|-------|------|----------|
| `status` | `"error"` | คงที่ |
| `timestamp` | ISO 8601 UTC | เวลาตอบกลับ |
| `code` | string | รหัส error (`E100_*` … `E600_*`) |
| `context` | string | กลุ่ม use case |
| `message` | string | ข้อความหลัก |
| `errors` | array | รายละเอียด field-level (ว่างได้ `[]`) |

**รูปแบบ `errors[]`**

| Field | คำอธิบาย |
|-------|----------|
| `field` | ชื่อ field ที่ผิด |
| `issue` | ประเภท เช่น `INVALID`, `REQUIRED`, `DUPLICATE`, `NOT_FOUND`, `XOR` |
| `message` | ข้อความอธิบาย |
| `code` | (บาง endpoint เช่น file upload) client error code เพิ่ม |
| อื่นๆ | blowoff close job อาจมี `sub_id`, `missing`, `point_bf_id` |

---

## 2. Response headers ทั่วไป

| Header | ค่า | หมายเหตุ |
|--------|-----|----------|
| `Content-Type` | `application/json; charset=utf-8` | JSON API |
| `X-App-Env` | `development` \| `staging` \| `production` | ทุก response ผ่าน gateway/services |
| `X-Staging-Mode` | `1` | เฉพาะ staging |
| `RateLimit-*` | ตาม express-rate-limit | เมื่อโดน rate limit |

**Authorization (protected routes)**

```
Authorization: Bearer <access_token>
```

---

## 3. Success codes ทั้งหมด

### 3.1 Auth / Session (`S200_*`)

| Code | Context | Message | HTTP | `data` ตัวอย่าง |
|------|---------|---------|------|----------------|
| `S200_001` | LOGIN | Login successful. | 200 | `{ user, roles, permissions, work_modules, access_token, refresh_token, token_type: "Bearer", expires_in }` |
| `S200_002` | LOGOUT | Logout successful. | 200 | `null` |
| `S200_003` | REFRESH_TOKEN | Token refreshed successfully. | 200 | `{ access_token, token_type, expires_in, roles, permissions, work_modules }` |
| `S200_004` | CHANGE_PASSWORD | Password changed successfully. | 200 | `null` |
| `S200_005` | VERIFY_TOKEN | Token is valid. | 200 | `{ user_id, roles, permissions, work_modules, expires }` |
| `S200_006` | PROFILE | Profile retrieved successfully. | 200 | `{ user, roles, permissions, work_modules }` |
| `S200_007` | REGISTER | Administrator account created successfully. | 201 | เหมือน login + `bootstrap: true` |

**Login / Register `data.user` (public)**

```json
{
  "id": 1,
  "employee_code": "EMP001",
  "title": "นาย",
  "first_name": "...",
  "last_name": "...",
  "full_name": "...",
  "email": "...",
  "position": "...",
  "active": true,
  "branch": { "id": 1, "code": "...", "name": "..." },
  "department": { "id": 1, "abbreviation_eng": "...", "abbreviation_th": "...", "name": "..." },
  "team": { "id": 1, "code": "...", "name": "..." },
  "photo_file_id": null,
  "signature_file_id": null
}
```

**`work_modules[]`**

```json
{ "code": "SURVEY", "name": "...", "service": "survey", "branch_scope": "ASSIGNED" }
```

---

### 3.2 CRUD / Action (`S201_*`)

| Code | Context | Message | HTTP | ใช้กับ |
|------|---------|---------|------|--------|
| `S201_001` | LIST | Records fetched successfully. | 200 | GET list, status-logs |
| `S201_002` | DETAIL | Record fetched successfully. | 200 | GET by id, profile, signed URL, dashboard, calculate |
| `S201_003` | CREATE | Record created successfully. | 201 | POST create, file upload, link image/video |
| `S201_004` | UPDATE | Record updated successfully. | 200 | PUT/PATCH, assign, change status |
| `S201_005` | DELETE | Record deleted successfully. | 200 | soft delete |
| `S201_006` | RECALCULATE | Recalculation completed successfully. | 200 | POST blowoff recalculate |

---

### 3.3 รูปแบบ `data` ตามประเภท endpoint

#### List (paginated)

```json
{
  "data": {
    "items": [ /* array of records */ ],
    "meta": {
      "page": 1,
      "limit": 20,
      "total": 100,
      "total_pages": 5,
      "has_next": true,
      "has_prev": false,
      "sort": "id",
      "order": "DESC"
    }
  }
}
```

Query รองรับ: `page`, `limit` (max 500), `sort`, `order`, `search`, filters ตาม module

#### List ไม่มี pagination (status logs)

```json
{
  "data": {
    "items": [ /* status log rows */ ]
  }
}
```

#### Detail (single record)

```json
{
  "data": { /* entity object */ }
}
```

#### Create / Update

```json
{
  "data": { /* entity object หลัง save */ }
}
```

#### Delete (soft delete)

```json
{
  "data": { "id": 123, "soft_deleted": true }
}
```

บาง module (เช่น user-role) อาจเป็น `{ "id": 123 }` เท่านั้น

#### Register status

```json
{
  "data": {
    "registration_open": false,
    "user_count": 5
  }
}
```

#### File upload (`POST /files/upload`)

```json
{
  "data": {
    "id": 1,
    "bucket": "ccm-mwa-storage",
    "file_key": "mwa/images/...",
    "file_name": "uuid.jpeg",
    "mime_type": "image/jpeg",
    "file_size_kb": 120,
    "width_px": null,
    "height_px": null,
    "duration_sec": null,
    "category": "LEAK_IMAGE",
    "is_active": true,
    "url": "https://signed-url..."
  }
}
```

#### File signed URL (`GET /files/:id/url`)

```json
{
  "data": {
    "id": 1,
    "url": "https://...",
    "expires_in": 86400
  }
}
```

---

## 4. Error codes ทั้งหมด + HTTP status

### 4.1 Validation (`E100_*`) → HTTP **400**

| Code | Context | Message |
|------|---------|---------|
| `E100_000` | VALIDATION | Invalid request body. |
| `E100_001` | VALIDATION | Invalid ID parameter. |
| `E100_002` | VALIDATION | Unknown fields in request. |
| `E100_003` | VALIDATION | Validation failed. |

**`E100_003` จาก express-validator** — `errors[]` มาจาก rule ที่ fail:

```json
{
  "field": "sub_date",
  "issue": "field",
  "message": "Invalid value"
}
```

**File upload validation** — ยังใช้ `E100_003` แต่ `message` / `errors` อาจเฉพาะ เช่น:

| Client code | message ตัวอย่าง |
|-------------|------------------|
| `CATEGORY_REQUIRED` | category required. Allowed: PROFILE, … |
| `INVALID_FILE_TYPE` | Allowed types: jpeg, png, webp, gif, pdf, mp4. |
| `VIDEO_MUST_BE_MP4` | Video must be MP4 … |
| `FILE_TOO_LARGE` | Max size: images 10 MB, video 25 MB. |
| `DURATION_REQUIRED` | Could not read MP4 duration … |
| `DURATION_OUT_OF_RANGE` | duration_sec must be between 0.5 and 15 seconds. |
| Missing `file` | Missing form field `file`. |

---

### 4.2 Auth (`E200_*`)

| Code | Context | Message | HTTP |
|------|---------|---------|------|
| `E200_001` | AUTH | Authorization token is required. | **401** |
| `E200_002` | AUTH | Invalid or malformed token. | **401** |
| `E200_003` | AUTH | Access token has expired. | **401** |
| `E200_004` | AUTH | Unauthorized. | **401** |
| `E200_005` | AUTH | Account is disabled. | **403** |
| `E200_006` | AUTH | User not found. | **401** / **404** (change password) |
| `E200_007` | AUTH | Insufficient permissions. | **403** |
| `E200_008` | AUTH | Invalid or expired refresh token. | **401** |
| `E200_009` | AUTH | Refresh token has been revoked. | **401** |
| `E200_010` | REGISTER | Registration is closed. Users already exist … | **403** |

**Login ผิด** — ใช้ business code แต่ HTTP **401**:

| Code | Message |
|------|---------|
| `E300_001` | Invalid credentials. |

(ทั้ง username ไม่พบและ password ผิด ใช้ code เดียวกัน)

---

### 4.3 Business (`E300_*`)

| Code | Context | Message | HTTP |
|------|---------|---------|------|
| `E300_001` | LOGIN | Invalid credentials. | 401 |
| `E300_002` | LOGIN | Invalid credentials. | (internal — map เป็น E300_001) |
| `E300_003` | CHANGE_PASSWORD | Current password is incorrect. | **400** |
| `E300_004` | BLOWOFF | End time must be after start time. | **400** |
| `E300_005` | BLOWOFF | Blowoff point not found. | **404** |
| `E300_006` | BLOWOFF | Blowoff job not found. | **404** |
| `E300_007` | BLOWOFF | department_id is required to generate the job code. | **400** |
| `E300_008` | BLOWOFF | branch_id is required … | **400** |
| `E300_009` | BLOWOFF | Team does not belong to the job department. | **400** |
| `E300_010` | SURVEY | Survey method not found. | **404** |
| `E300_011` | SURVEY | Job status cannot be changed via update. Use the status endpoint. | **400** |
| `E300_012` | SURVEY | A reason is required when changing job status. | **400** |
| `E300_013` | SURVEY | Worker not found. | **404** |
| `E300_014` | SURVEY | Severity must be an integer from 1 to 5. | **400** |
| `E300_015` | SURVEY | Image must be linked to exactly one of survey_finding_id or survey_sub_id. | **400** |
| `E300_016` | SURVEY | Survey job not found. | **404** |
| `E300_017` | REPAIR | Repair job not found. | **404** |
| `E300_018` | REPAIR | Repair item catalog entry not found. | **404** |
| `E300_019` | REPAIR | Provide either catalog_id or equipment_rate_item_id, not both. | **400** |
| `E300_020` | REPAIR | Equipment rate item not found. | **404** |
| `E300_021` | REPAIR | No equipment rate effective on the given date. | **404** |
| `E300_022` | REPAIR | Equipment rate order not found. | **404** |
| `E300_023` | REPAIR | Equipment rate order is cancelled. | **400** |
| `E300_024` | REPAIR | An open rate already exists for this equipment item. | **409** |
| `E300_025` | REPAIR | Invalid multiplier rule code. | **400** |
| `E300_026` | REPAIR | used_date is required for equipment rental items. | **400** |
| `E300_027` | JOB | assigned_team_id is required for assignment. | **400** |
| `E300_028` | JOB | Selected user is not an active member of the team. | **400** |
| `E300_029` | REPAIR | This survey finding has already been referred to a repair job. | **409** |
| `E300_030` | BLOWOFF | Cannot close blowoff job: one or more sub-jobs have incomplete water-loss calculation. | **400** |
| `E300_031` | BLOWOFF | Blowoff point gate_size (mm) is required before water-loss can be calculated. | **400** |

**`E300_030` errors[] ตัวอย่าง**

```json
{
  "field": "sub_id",
  "issue": "INCOMPLETE_CALC",
  "message": "Sub 12 missing: flow_pct, duration_min",
  "sub_id": 12,
  "missing": ["flow_pct", "duration_min"]
}
```

> หมายเหตุ: Survey / Repair / Blowoff job status ใช้ `E300_011`, `E300_012` ร่วมกัน (context อาจเป็น SURVEY หรือ BLOWOFF ตาม module)

---

### 4.4 Staging (`E600_*`) → HTTP **423**

| Code | Context | Message |
|------|---------|---------|
| `E600_001` | STAGING_GUARD | This operation is disabled on the Staging environment to protect production data. |

---

### 4.5 Database / Domain (`E400_*`)

| Code | Context | Message | HTTP |
|------|---------|---------|------|
| `E400_001` | DATABASE | Duplicate record. | **409** |
| `E400_002` | DATABASE | Record not found. | **404** |
| `E400_003` | DATABASE | Cannot delete: record is referenced. | **409** |
| `E400_004` | USERS | Employee code already exists. | **409** |
| `E400_005` | USERS | Email already exists. | **409** |
| `E400_006` | USERS | Branch not found. | **404** |
| `E400_007` | USERS | Department not found. | **404** |
| `E400_008` | USERS | Team not found. | **404** |
| `E400_009` | RBAC | Role not found. | **404** |
| `E400_010` | RBAC | Permission not found. | **404** |
| `E400_011` | FILE_STORAGE | File not found. | **404** |
| `E400_012` | REPAIR | Contract not found. | **404** |

**Duplicate job codes** (SV/BF/RP) map เป็น `E400_001` HTTP **409**

---

### 4.6 System (`E500_*`)

| Code | Context | Message | HTTP |
|------|---------|---------|------|
| `E500_001` | SYSTEM | Internal server error. | **500** |
| `E500_002` | SYSTEM | Endpoint not found. | **404** |
| `E500_003` | GATEWAY | Upstream service is unavailable. | **502** |
| `E500_004` | SYSTEM | Database schema is outdated. Apply pending migrations … | **503** |

**MySQL errors (global error handler)**

| MySQL | Maps to | HTTP |
|-------|---------|------|
| `ER_DUP_ENTRY` | `E400_001` | 409 |
| `ER_ROW_IS_REFERENCED_2` / `ER_NO_REFERENCED_ROW_2` | `E400_003` | 409 |
| `ER_BAD_FIELD_ERROR` / `ER_NO_SUCH_TABLE` | `E500_004` | 503 |
| อื่นๆ | `E500_001` | 500 |

---

## 5. Response พิเศษ (ไม่ใช่ envelope มาตรฐาน)

### 5.1 Rate limit → HTTP **429**

Gateway `express-rate-limit` — **ไม่มี** `code` / `timestamp`:

```json
{ "status": "error", "message": "Too many requests." }
```

Auth routes:

```json
{ "status": "error", "message": "Too many authentication attempts." }
```

---

### 5.2 Gateway health → `GET /health`

```json
{
  "status": "ok",
  "env": "production",
  "services": [
    { "name": "auth", "status": "ok" },
    { "name": "blowoff", "status": "down" }
  ]
}
```

| `status` | HTTP |
|----------|------|
| `"ok"` (ทุก service ok) | **200** |
| `"degraded"` | **503** |

---

### 5.3 Gateway root → `GET /`

```json
{
  "name": "MWA API Gateway",
  "env": "production",
  "api_prefix": "/api/mwa/v1",
  "socket_io_path": "/api/mwa/v1/socket.io",
  "services": [
    { "name": "auth", "target": "http://127.0.0.1:5012", "segments": ["auth"] }
  ]
}
```

---

### 5.4 File stream → `GET /files/:id/view` | `/download` | `/files/view?key=`

**ไม่ใช่ JSON** — stream binary จาก S3

| Header | ค่า |
|--------|-----|
| `Content-Type` | mime จาก S3 |
| `Content-Disposition` | `inline` หรือ `attachment` |

ถ้า error **ก่อน** stream เริ่ม → JSON envelope error (`E400_011`, `E500_001`)

---

## 6. สรุป HTTP status ที่ client ควร handle

| HTTP | ความหมาย | ตรวจ |
|------|----------|------|
| 200 | OK | `status === "success"` |
| 201 | Created | `status === "success"` |
| 400 | Validation / business rule | `code` ขึ้นต้น `E100_`, `E300_` |
| 401 | ไม่มี token / token หมดอายุ / login ผิด | `E200_*`, `E300_001` |
| 403 | สิทธิ์ไม่พอ / account ปิด / register ปิด | `E200_005`, `E200_007`, `E200_010` |
| 404 | ไม่พบ record / endpoint | `E400_002`, `E300_*_NOT_FOUND`, `E500_002` |
| 409 | ซ้ำ / FK / already referred | `E400_001`, `E400_003`, `E300_029`, `E300_024` |
| 423 | Staging block | `E600_001` |
| 429 | Rate limit | `message` only |
| 500 | Server error | `E500_001` |
| 502 | Gateway upstream down | `E500_003` |
| 503 | Schema outdated / health degraded | `E500_004` |

---

## 7. แนวทาง client (แนะนำ)

```javascript
async function callApi(path, options = {}) {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  // File stream endpoints — ไม่ parse JSON
  if (path.includes("/view") || path.includes("/download")) {
    if (!res.ok) throw await res.json();
    return res.blob();
  }

  const body = await res.json();

  if (body.status === "error") {
    // ใช้ body.code เป็นหลัก — ไม่ใช้ message อย่างเดียว
    throw { httpStatus: res.status, ...body };
  }

  return body.data;
}
```

**Refresh token flow**

1. Login → เก็บ `access_token` + `refresh_token`
2. API 401 + `E200_003` → `POST /auth/refresh` with `refresh_token`
3. 401 + `E200_008` / `E200_009` → redirect login

---

## 8. ไฟล์อ้างอิงใน repo

| ไฟล์ | เนื้อหา |
|------|---------|
| `shared/constants/errorCodes.js` | error codes ทั้งหมด |
| `shared/constants/successCodes.js` | success codes ทั้งหมด |
| `shared/utils/sendSuccess.js` | success envelope |
| `shared/utils/sendError.js` | error envelope |
| `shared/middleware/validate.js` | validation → `E100_003` |
| `shared/middleware/errorHandler.js` | MySQL / 404 fallback |
| `shared/config/security.js` | rate limit message |
| `gateway/server.js` | health, 502, 404 gateway |

---

*อัปเดตตาม codebase SOA v2 — ถ้าเพิ่ม error code ใหม่ ให้ sync ที่ `shared/constants/errorCodes.js` และเอกสารนี้*
