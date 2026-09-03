# RTU API — Contract / Documentation

เอกสารนี้อธิบาย **เส้น API ที่ frontend ต้องเรียก** พร้อม field ครบ เงื่อนไข และ error ที่พบบ่อย  
Base path เริ่มต้น: `{base_url}{api_prefix}` เช่น `https://host/api/rtu/v1`

---

## สารบัญ

| ไฟล์ | เนื้อหา |
|------|---------|
| [00-conventions.md](./00-conventions.md) | Envelope, pagination, PATCH semantics, auth |
| [01-work-orders.md](./01-work-orders.md) | PM/CM ใบงาน — **อ่านไฟล์นี้ก่อนถ้าทำงานใบงาน** |
| [02-pm-reports.md](./02-pm-reports.md) | รายงาน PM, onsite-fix, escalate |
| [03-cm-reports.md](./03-cm-reports.md) | รายงาน CM, duplicate, problem topic |
| [04-panels-devices.md](./04-panels-devices.md) | ตู้, อุปกรณ์ในตู้, รูปภาพ |
| [05-calibrations.md](./05-calibrations.md) | สอบเทียบ, เครื่องมือ, readings |
| [06-masters.md](./06-masters.md) | Engineers, checklist, problem-topics, device-models |
| [07-attachments-notifications.md](./07-attachments-notifications.md) | ไฟล์แนบ, แจ้งเตือน |

Postman: `postman/RTU-API.postman_collection.json`  
Error codes เต็ม: `api-response-reference.md` (repo root)

---

## แผนที่ “อยากทำ X → เรียกเส้นไหน”

### ใบงาน PM / CM

| อยากทำ | Method | Path |
|--------|--------|------|
| สร้างใบงาน | `POST` | `/work-orders` หรือ `/panels/{panel_id}/work-orders` |
| ดูรายละเอียดใบ | `GET` | `/work-orders/{id}` |
| แก้ชื่อ/วันที่/priority/schedule | `PATCH` | `/work-orders/{id}` — ดู [01-work-orders.md § PATCH](./01-work-orders.md#patch--put-work-ordersid) |
| เปลี่ยนผู้รับผิดชอบ | `POST` | `/work-orders/{id}/reassign` |
| เริ่มงานหน้างาน | `POST` | `/work-orders/{id}/check-in` |
| ออกจากหน้างาน | `POST` | `/work-orders/{id}/check-out` |
| บันทึกรายงาน PM (draft) | `PUT` | `/work-orders/{id}/pm-report` |
| ส่งรายงาน PM อนุมัติ | `POST` | `/work-orders/{id}/pm-report/submit` |
| บันทึกรายงาน CM | `PUT` | `/work-orders/{id}/cm-report` |
| ส่งรายงาน CM อนุมัติ | `POST` | `/work-orders/{id}/cm-report/submit` |
| อนุมัติ/ปฏิเสธ | `POST` | `/work-orders/{id}/approvals` |
| ยกเลิกใบ (soft) | `DELETE` | `/work-orders/{id}` |
| ดู CM เปิดอยู่บนตู้เดียวกัน | `GET` | `/work-orders/{id}/open-cm-work-orders` หรือ `/panels/{id}/open-cm-work-orders` |

### ระหว่างทำ PM

| อยากทำ | Method | Path |
|--------|--------|------|
| แก้ปัญหา onsite (จบในวัน) | `POST` | `/pm-reports/{pm_report_id}/onsite-fixes` |
| แจ้งปัญหา spawn CM | `POST` | `/pm-reports/{pm_report_id}/escalate` |
| ดูรายงาน PM ฉบับเต็ม | `GET` | `/pm-reports/{id}` หรือ `/work-orders/{id}/pm-report` |

### CM

| อยากทำ | Method | Path |
|--------|--------|------|
| สร้าง CM ใหม่ | `POST` | `/work-orders` + `work_order_type=CM` + **topic อย่างน้อย 1** — `problem_topic_id` (UUID เดียว) หรือ `problem_topic_ids` (array) |
| แก้รายงาน CM โดยตรง | `PATCH` | `/cm-reports/{id}` |
| ดูหัวข้อปัญหา (pill UI) | `GET` | `/problem-topics?active=true` |

---

## Workflow สถานะใบงาน (สรุป)

สถานะใบงาน (`work_order.status`) มีได้แค่:  
`ASSIGNED` → `IN_PROGRESS` → `PENDING_APPROVAL` → `COMPLETED` / `CONDITIONAL` / `CANCELLED`  
และ `PENDING` (rework หลัง reject เท่านั้น)

```
สร้างใบ ──► ASSIGNED ──check-in──► IN_PROGRESS ──check-out──► IN_PROGRESS (ยัง)
                                      │                              │
                                      │                    submit report
                                      │                              ▼
                                      │                    PENDING_APPROVAL
                                      │                              │
                                      │              POST .../approvals (review)
                                      │                              │
                                      │        ┌─────────────────────┼─────────────────────┐
                                      │        ▼                     ▼                     ▼
                                      │   COMPLETED           CONDITIONAL     decision=REJECTED
                                      │                                              │
                                      │                              ┌───────────────┴───────────────┐
                                      │                              ▼                               ▼
                                      │                    rework → PENDING              escalate → CONDITIONAL
                                      │                    (round ใหม่)                  + spawn/reuse CM
                                      │                              │
                                      └──────────────── check-in ────┘
```

**หมายเหตุสำคัญ**
- **ไม่มี status `REJECTED` บนใบงาน** — `REJECTED` เป็น `decision` ใน body ของ `POST .../approvals` เท่านั้น
- **check-out ไม่เปลี่ยน status** — ยัง `IN_PROGRESS` จนกว่าจะ submit report → `PENDING_APPROVAL`
- **`PENDING`** = รอ rework หลัง reject (round ใหม่) — **ไม่ใช่** หลัง check-out
- **สถานะเปลี่ยนผ่าน action endpoint เท่านั้น** — ห้ามส่ง `status` ใน PATCH ใบงาน

---

## กฎสำคัญสำหรับ Frontend

1. **PATCH ใบงาน** — ส่งเฉพาะ field ที่จะเปลี่ยน; field read-only จาก GET ส่งมาได้ (BindLenient) แต่จะถูกละเว้น; ส่ง**เฉพาะ** read-only → `400 E100_003`
2. **PUT รายงานใต้ work-order** — `PUT .../pm-report` และ `PUT .../cm-report` ส่ง body ครบทุกครั้ง (ทุก field มีผล รวม `null` = ล้าง); **ไม่ใช่** partial แบบ `PATCH /cm-reports/{id}`
3. **PM report save** — ใช้ได้แค่ตอน report ยัง `DRAFT`; หลัง submit แล้ว → `409 E300_217`
4. **CM create** — `problem_topic_id` บังคับ; ซ้ำ panel+topic ขณะเปิด → `409 E300_246`
5. **วันที่** — ใช้ `"YYYY-MM-DD"` สำหรับ `planned_date`, `due_date`, `repair_date`
6. **UUID** — string รูปแบบมาตรฐาน เช่น `"4bccb3c4-899d-439a-9423-f782f8ba4f52"`
