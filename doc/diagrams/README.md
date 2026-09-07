# RTU PM/CM Workflow Diagrams (draw.io)

ชุด flow diagram ระดับ senior สำหรับทุกงานในโดเมน PM/CM — อ้างอิงจาก `doc/api/*.md` และ implementation จริงใน `internal/service/`

## ไฟล์หลัก (แนะนำ)

| ไฟล์ | คำอธิบาย |
|------|----------|
| **[rtu-pm-cm-workflows.drawio](./rtu-pm-cm-workflows.drawio)** | **ไฟล์รวม 7 หน้า (tabs)** — เปิดไฟล์เดียวใน draw.io แล้วสลับ tab ด้านล่าง |
| [pm-repair-during-pm-flow.drawio](./pm-repair-during-pm-flow.drawio) | หน้าเดียว — zoom ลึกเฉพาะ onsite fix vs escalate (legacy / ใช้แยกนำเสนอได้) |

## วิธีเปิด

1. [https://app.diagrams.net](https://app.diagrams.net) → **File → Open** → เลือก `rtu-pm-cm-workflows.drawio`
2. สลับ **tab** ด้านล่าง: `00 — Overview` … `06 — Approval`
3. Export PNG/PDF: **File → Export as**

## Regenerate จาก code

```bash
node scripts/generate_workflow_diagrams.mjs
```

แก้ logic / ข้อความใน `scripts/generate_workflow_diagrams.mjs` แล้วรันคำสั่งด้านบน

---

## สารบัญ tabs (rtu-pm-cm-workflows.drawio)

### 00 — Overview

ภาพรวม **งานทั้งหมด** ในระบบ:

| งาน | มี Work Order? | API หลัก |
|-----|----------------|----------|
| PM Work Order | ✓ `PM-...` | `POST /work-orders` (PM) |
| CM Standalone | ✓ `CM-...` | `POST /work-orders` (CM) |
| Onsite Fix | ✗ | `POST /pm-reports/{id}/onsite-fixes` |
| Escalate ระหว่าง PM | ✓ (spawn CM) | `POST /pm-reports/{id}/escalate` |
| Approval Escalate | ✓ (spawn/reuse CM) | `POST /work-orders/{id}/approvals` |

### 01 — PM Work Order

วงจร PM เต็ม: create → reassign → check-in → save PM report → (repair during PM) → submit → approval → terminal / rework loop

**Submit validation:**

- `THREE_MONTH` → ต้องมี `power_test`
- `SIX_MONTH` → ต้องมี calibration ≥ 1 ผูก PM

### 02 — CM Work Order

วงจร CM แบบ Standalone (STANDALONE origin): create + duplicate guard → check-in → save CM report → submit → approval

รองรับ multi-topic ผ่าน `work_order_problem_topics`

### 03 — Status State Machine

`work_orders.status` ทุก transition — **ห้าม PATCH status**

```
ASSIGNED → IN_PROGRESS → PENDING_APPROVAL → COMPLETED / CONDITIONAL
                              ↓ REJECTED
                           PENDING → (round ใหม่) → IN_PROGRESS
```

**หมายเหตุ:** check-out **ไม่เปลี่ยน** status · `REJECTED` เป็น `decision` ใน approvals ไม่ใช่ WO status

### 04 — Repair During PM

Decision tree ระหว่างทำ PM:

- **ซ่อมได้** → onsite fix (`PM_ONSITE_FIX`, ไม่มี CM WO)
- **ซ่อมไม่ได้** → escalate spawn CM (`PM_ESCALATED`)

Precondition: PM WO + check-in + มี PM report draft

### 05 — CM Report Origins

สามต้นทาง `cm_reports` (คำนวณจาก FK):

| Origin | work_order_id | pm_report_id | CM WO |
|--------|---------------|--------------|-------|
| STANDALONE | ✓ | ✗ | ✓ |
| PM_ONSITE_FIX | ✗ | ✓ | ✗ |
| PM_ESCALATED | ✓ | ✓ | ✓ |

### 06 — Approval

`POST .../approvals` เมื่อ `PENDING_APPROVAL`:

- `APPROVED` / `APPROVED_CONDITION`
- `REJECTED` → rework (PENDING + round ใหม่)
- `REJECTED` + `escalate=true` → spawn/reuse CM (ต่างจาก escalate หน้างาน)

---

## สีใน diagram

| สี | ความหมาย |
|----|----------|
| 🔵 น้ำเงิน | PM work order / PM actions |
| 🔴 แดง | CM work order / CM actions |
| 🟢 เขียว | จบสำเร็จ / onsite fix (ไม่มี WO) |
| 🟠 ส้ม | Decision / warning / validation |
| 🟡 เหลือง | Note / API reference |

---

## เอกสารที่เกี่ยวข้อง

- [doc/api/README.md](../api/README.md) — API map
- [doc/api/01-work-orders.md](../api/01-work-orders.md) — WO lifecycle
- [doc/api/02-pm-reports.md](../api/02-pm-reports.md) — PM report, onsite, escalate
- [doc/api/03-cm-reports.md](../api/03-cm-reports.md) — CM report, duplicate
- [postman/RTU-API.postman_collection.json](../../postman/RTU-API.postman_collection.json) — `02 — PM Smoke Flow`
