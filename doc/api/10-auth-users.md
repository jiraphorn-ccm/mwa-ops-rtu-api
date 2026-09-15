# 10 — Auth / Users / Audit

ไม่มี RBAC — รู้แค่ว่า **ใคร login** และ **ใครทำ action ไหน**  
Access token อายุ **15 นาที** (`JWT_ACCESS_EXPIRES`) · refresh **7 วัน** (`JWT_REFRESH_EXPIRES_DAYS`) · HS256

ทุกเส้นใต้ `{api_prefix}` **ต้องมี Bearer** ยกเว้นตารางด้านล่าง

---

## สาธารณะ (ไม่ต้องมี token)

| Method | Path | หมายเหตุ |
|--------|------|----------|
| `POST` | `/auth/register` | ได้เฉพาะตอนยังไม่มี user ในระบบ |
| `GET` | `/auth/register/status` | `registration_open` |
| `POST` | `/auth/login` | `username` = email หรือ `employee_code` |
| `POST` | `/auth/refresh` | ออก access token ใหม่ จาก refresh |
| `POST` | `/auth/logout` | revoke refresh (ไม่ error ถ้า token หาย) |
| `GET` | `/health` `/health/live` `/health/ready` | นอก prefix |

อื่น ๆ ถ้าไม่มี `Authorization: Bearer <access_token>` → `401` `E200_001`

---

## Auth

### `GET /auth/register/status`

```json
{ "registration_open": true, "user_count": 0 }
```

### `POST /auth/register` — คนแรกเท่านั้น

| Field | บังคับ |
|-------|--------|
| `employee_code` | ✅ max 20 unique |
| `first_name` | ✅ |
| `last_name` | ✅ |
| `email` | ✅ unique (เก็บเป็นตัวพิมพ์เล็ก) |
| `password` | ✅ min 8, max 72 |
| `title` | ไม่ |
| `position` | ไม่ |

มี user อยู่แล้ว → `403` `E200_010`

Response เหมือน login (`access_token`, `refresh_token`, `user`)

### `POST /auth/login`

```json
{ "username": "somchai@example.com", "password": "secret123" }
```

`username` เป็น **email หรือ employee_code**

```json
{
  "user": { "id": "…", "employee_code": "E001", "full_name": "Somchai Jaidee", "email": "…" },
  "access_token": "<jwt>",
  "refresh_token": "<uuid>",
  "token_type": "Bearer",
  "expires_in": 900
}
```

`expires_in` เป็นวินาที (15 นาที = 900) · **ไม่คืน password**

| สถานะ | Code |
|--------|------|
| username/password ผิด | `401` `E300_001` |
| บัญชีถูกปิด | `403` `E200_005` |

### `POST /auth/refresh`

```json
{ "refresh_token": "{{refresh_token}}" }
```

คืน `access_token` ใหม่ (refresh เดิมยังใช้ได้จนกว่าจะหมดอายุ/revoke)

| สถานะ | Code |
|--------|------|
| ไม่รู้จัก / หมดอายุ | `401` `E200_008` |
| ถูก revoke (logout / เปลี่ยนรหัส) | `401` `E200_009` |

### `POST /auth/logout`

```json
{ "refresh_token": "{{refresh_token}}" }
```

### `GET /auth/me` — ต้องมี Bearer

โปรไฟล์ของผู้ถือ token

### `POST /auth/change-password` — ต้องมี Bearer

```json
{ "old_password": "secret123", "new_password": "newsecret1" }
```

สำเร็จแล้ว **revoke refresh ทุก session** ของ user นั้น

รหัสเก่าผิด → `400` `E200_011`

---

## Users

Prefix: `{api_prefix}/users` · **ต้องมี Bearer ทุกเส้น** · ไม่มีสิทธิ์แยก (ใคร login ก็เรียกได้)

| Method | Path |
|--------|------|
| `GET` / `POST` | `/users` |
| `GET` / `PATCH` / `PUT` / `DELETE` | `/users/{id}` |
| `POST` | `/users/{id}/restore` |
| `DELETE` | `/users/{id}/permanent` |

### POST — `UserCreateInput`

| Field | บังคับ |
|-------|--------|
| `employee_code` | ✅ max 20 unique |
| `first_name` | ✅ |
| `last_name` | ✅ |
| `email` | ✅ unique |
| `password` | ✅ min 8 |
| `title` | ไม่ max 10 |
| `position` | ไม่ |
| `active` | bool default true |

### PATCH — `UserUpdateInput`

ฟิลด์ด้านบนทั้งหมดเป็น partial รวม `password` (reset โดยไม่ต้องใส่รหัสเก่า — ไม่ revoke session; ใช้ `/auth/change-password` ถ้าต้องการตัด session)

**List filter:** `active`, `search` (code / email / ชื่อ) · paginated · sort: `employee_code`, `first_name`, `last_name`, `email`, `active`, `created_at`, `updated_at`

Response มี `full_name` รวม title+ชื่อ · **ไม่มี `password_hash`**

`DELETE /users/{id}` = soft (`active=false`)

---

## Audit logs

`GET /audit-logs` — ต้องมี Bearer

ทุก `POST`/`PUT`/`PATCH`/`DELETE` (ยกเว้น login/refresh/logout/register ซึ่งบันทึกจาก auth service) ถูกเขียนลง `rtu.audit_logs`

| Query | ความหมาย |
|-------|----------|
| `user_id` | UUID ผู้กระทำ |
| `action` | `CREATE` `UPDATE` `DELETE` `RESTORE` `PURGE` `CHECK_IN` `LOGIN` … |
| `resource` | เช่น `panels`, `users` |
| `from` / `to` | RFC 3339 หรือ `YYYY-MM-DD` |
| `search` | path / action |

แต่ละแถวมี `user_id`, `action`, `method`, `path`, `resource`, `resource_id`, `status_code`, `ip_address`, `actor_employee_code`, `actor_first_name`, `actor_last_name`

คอลัมน์ `created_by` / `updated_by` บนตารางอื่น ถูกเติมจาก JWT `user_id` อัตโนมัติเมื่อมี token

---

## Error codes (auth / users)

| Code | HTTP | ความหมาย |
|------|------|----------|
| `E200_001` | 401 | ไม่มี Bearer |
| `E200_002` | 401 | token ผิดรูปแบบ / ไม่ใช่ access |
| `E200_003` | 401 | access หมดอายุ (15 นาที) — ใช้ refresh |
| `E200_005` | 403 | บัญชีถูกปิด |
| `E200_008` | 401 | refresh ไม่ถูกต้องหรือหมดอายุ |
| `E200_009` | 401 | refresh ถูก revoke |
| `E200_010` | 403 | register ปิดแล้ว |
| `E200_011` | 400 | รหัสผ่านเดิมไม่ถูก |
| `E300_001` | 401 | login ผิด |
| `E300_249` | 404 | ไม่พบ user |
| `E300_250` | 409 | email ซ้ำ |
| `E300_251` | 409 | employee_code ซ้ำ |
