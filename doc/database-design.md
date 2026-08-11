# RTU Database Design

เอกสารสรุปโครงสร้างฐานข้อมูล RTU — อ้างอิงจาก:

- [`Database Design.docx.pdf`](./Database%20Design.docx.pdf) v1.2 (PM/CM Platform ทั้งระบบ)
- [`System Design.pdf`](./System%20Design.pdf) (Use cases, Screens, Workflow)
- **Implementation จริง:** `migrations/` ใน repo นี้ (RTU Calibration API)

> ไฟล์ DBML สำหรับ import dbdiagram.io: [`rtu-calibration-schema.dbml`](./rtu-calibration-schema.dbml)

---

## Scrutinize Summary

**เป้าหมาย Design Doc:** ฐานข้อมูล PM/CM ครบวงจร (~25 ตาราง) — users, work_order, pm_report, checklist, ground test, power test, CM, approval, notification

**เป้าหมาย repo นี้:** API ชั้น **Calibration domain** ใน schema `rtu` (6 ตาราง) — master ตู้/อุปกรณ์ + ใบสอบเทียบ standalone

**Verdict: ship (เอกสารนี้)** — แยก scope ชัด: ไฟล์ใน `doc/` อธิบาย **schema ที่ implement แล้ว** + แผนที่ไป Design Doc ทั้งระบบ

| Finding | Why it matters | Evidence |
|---------|----------------|----------|
| **Major** — Design Doc กับ schema `rtu` ไม่ใช่ชุดเดียวกัน | Dev อาจคิดว่า migration นี้ครอบ PM/CM ทั้งหมด | Design Doc §3 มี `work_order`, `pm_report`; repo มีแค่ `rtu.calibrations` |
| **Major** — PK type ต่างกัน | ไม่ merge DB โดยตรงได้ | Design: `SERIAL int`; Implementation: `uuid` + `gen_random_uuid()` |
| **Major** — `pm_calibration_point` รวมกว่า `calibration_readings` | PM6 ต้องเก็บ as_found/as_left แยกคอลัมน์ — schema นี้ใช้ key-value ยืดหยุ่น | System Design §Calibration Test Points vs `parameter_key` + `value` |
| **Nit** — `performed_by` ≠ `created_by` | คนสอบเทียบ (ชื่อ) vs คน login บันทึกระบบ (JWT id) | `calibrations.performed_by` + `created_by` แยกกัน |

---

## 1. ภาพรวม Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  MWA Platform (Design Doc v1.2 — อนาคต / service อื่น)          │
│  users, work_order, pm_report, cm_report, checklist, ...        │
│  schema: public (int PK)                                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │ อ้างอิง master / ซิงค์ข้อมูล
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  RTU Calibration API (repo นี้)                                 │
│  schema: rtu (uuid PK)                                          │
│  panels → panel_devices → calibrations → calibration_readings   │
│  device_models, calibration_instruments                         │
└─────────────────────────────────────────────────────────────────┘
```

### ER Diagram (Implemented)

```
rtu.device_models ──< rtu.panel_devices >── rtu.panels
                           │
                           └──< rtu.calibrations >── rtu.calibration_instruments
                                     │
                                     └──< rtu.calibration_readings
```

---

## 2. Traceability — Design Doc → Schema `rtu`

| Design Doc Table | Screen / Domain | Schema `rtu` (implemented) | หมายเหตุ |
|------------------|-----------------|---------------------------|----------|
| `rtu` | Web 09, PM/CM | **`rtu.panels`** | `rtu_code`→`code`, `station_name`→`location` |
| `equipment` | Web 09, PM Checklist | **`rtu.panel_devices`** + **`rtu.device_models`** | แยก master รุ่น vs instance |
| `pm_calibration` | Mobile PM §D | **`rtu.calibrations`** | ไม่มี `channel_type`, `pass_fail` แยก — ใช้ `result` |
| `pm_calibration_point` | Mobile PM §C | **`rtu.calibration_readings`** | generic key/value แทนคอลัมน์ as_found |
| `pm_calibration.ref_*` | Calibration §B | **`rtu.calibration_instruments`** | เครื่องมือมาตรฐานอ้างอิง |
| `users` | Auth | *(MWA auth service)* | JWT `user_id` → `created_by`/`updated_by` |
| `work_order` | Web 03 | *(ยังไม่ implement)* | ใบงาน PM/CM |
| `pm_report` | Mobile PM 03 | *(ยังไม่ implement)* | รายงาน PM ครบ checklist |
| `checklist_result` | PM Checklist | *(ยังไม่ implement)* | 13 ข้อตรวจ |
| `pm_ground_test` | PM Ground | *(ยังไม่ implement)* | |
| `pm_power_test` | PM Power | *(ยังไม่ implement)* | |
| `cm_report` | Mobile CM 04 | *(ยังไม่ implement)* | |

---

## 3. ตารางทั้งหมด (Implemented) — DBML Style

### 3.1 `rtu.panels`

```dbml
Table rtu.panels {
  id uuid [pk, default: `gen_random_uuid()`, note: 'รหัสประจำตู้ RTU']
  code varchar(20) [not null, unique, note: 'รหัสตู้ RTU']
  location text [note: 'รายละเอียดตำแหน่งติดตั้งตู้ RTU']
  latitude numeric(10,7) [note: 'ละติจูดของตำแหน่งติดตั้ง']
  longitude numeric(10,7) [note: 'ลองจิจูดของตำแหน่งติดตั้ง']
  active boolean [not null, default: true, note: 'สถานะการใช้งานของตู้']
  created_at timestamptz [not null, default: `now()`, note: 'วันที่สร้างข้อมูล']
  updated_at timestamptz [not null, default: `now()`, note: 'วันที่แก้ไขข้อมูลล่าสุด']
  created_by varchar(100) [note: 'ผู้สร้าง (JWT user_id)']
  updated_by varchar(100) [note: 'ผู้แก้ไขล่าสุด (JWT user_id)']

  Indexes {
    code [unique, name: "uk_panels_code"]
    active [name: "idx_panels_active"]
    created_at [name: "idx_panels_created_at"]
  }
}
```

**Constraints:** `ck_panels_latitude` (−90…90), `ck_panels_longitude` (−180…180)  
**Trigger:** `trg_panels_updated_at` → `rtu.set_updated_at()`

---

### 3.2 `rtu.device_models`

```dbml
Table rtu.device_models {
  id uuid [pk, default: `gen_random_uuid()`, note: 'รหัสรุ่นอุปกรณ์']
  code varchar(30) [not null, unique, note: 'รหัสอ้างอิงรุ่นอุปกรณ์']
  name varchar(100) [not null, note: 'ชื่อประเภทอุปกรณ์']
  manufacturer varchar(100) [note: 'ผู้ผลิตอุปกรณ์']
  model varchar(100) [note: 'ชื่อรุ่นอุปกรณ์']
  description text [note: 'รายละเอียดเพิ่มเติมของรุ่นอุปกรณ์']
  active boolean [not null, default: true, note: 'สถานะการใช้งานของรุ่นอุปกรณ์']
  created_at timestamptz [not null, default: `now()`]
  updated_at timestamptz [not null, default: `now()`]
  created_by varchar(100)
  updated_by varchar(100)

  Indexes {
    code [unique, name: "uk_device_model_code"]
    active [name: "idx_device_models_active"]
  }
}
```

---

### 3.3 `rtu.panel_devices`

```dbml
Table rtu.panel_devices {
  id uuid [pk, default: `gen_random_uuid()`, note: 'รหัสอุปกรณ์ที่ติดตั้ง']
  panel_id uuid [not null, note: 'อ้างอิงตู้ RTU ที่ติดตั้ง']
  device_model_id uuid [not null, note: 'อ้างอิงรุ่นอุปกรณ์']
  tag_name varchar(100) [note: 'Tag Name หรือชื่ออุปกรณ์ภายในตู้']
  serial_number varchar(100) [note: 'Serial Number ของอุปกรณ์']
  asset_code varchar(100) [note: 'รหัสครุภัณฑ์หรือทรัพย์สิน']
  firmware_version varchar(50) [note: 'เวอร์ชัน Firmware']
  communication_status varchar(20) [not null, default: 'UNKNOWN', note: 'ONLINE|OFFLINE|DEGRADED|UNKNOWN']
  health_status varchar(20) [not null, default: 'UNKNOWN', note: 'NORMAL|WARNING|CRITICAL|UNKNOWN']
  installed_at date [note: 'วันที่ติดตั้งอุปกรณ์']
  last_seen_at timestamptz [note: 'เวลาที่อุปกรณ์ส่งข้อมูลล่าสุด']
  note text [note: 'หมายเหตุเพิ่มเติม']
  active boolean [not null, default: true, note: 'สถานะการใช้งานของอุปกรณ์']
  created_at timestamptz [not null, default: `now()`]
  updated_at timestamptz [not null, default: `now()`]
  created_by varchar(100)
  updated_by varchar(100)

  Indexes {
    panel_id [name: "idx_panel_devices_panel_id"]
    device_model_id [name: "idx_panel_devices_device_model_id"]
    (panel_id, device_model_id) [name: "idx_panel_devices_panel_model"]
    active [name: "idx_panel_devices_active"]
    last_seen_at [name: "idx_panel_devices_last_seen_at"]
    serial_number [unique, name: "uk_device_serial"]
    (panel_id, tag_name) [unique, name: "uk_panel_device_tag", note: 'WHERE tag_name IS NOT NULL']
  }
}
```

---

### 3.4 `rtu.calibration_instruments`

```dbml
Table rtu.calibration_instruments {
  id uuid [pk, default: `gen_random_uuid()`, note: 'รหัสเครื่องมือสอบเทียบ']
  name varchar(100) [not null, note: 'ชื่อเครื่องมือสอบเทียบ']
  manufacturer varchar(100) [note: 'ผู้ผลิตเครื่องมือ']
  model varchar(100) [note: 'รุ่นของเครื่องมือ']
  serial_number varchar(100) [unique, note: 'Serial Number ของเครื่องมือ']
  calibration_date date [note: 'วันที่สอบเทียบล่าสุด']
  expire_date date [note: 'วันที่หมดอายุการสอบเทียบ']
  active boolean [not null, default: true, note: 'สถานะการใช้งานของเครื่องมือ']
  created_at timestamptz [not null, default: `now()`]
  updated_at timestamptz [not null, default: `now()`]
  created_by varchar(100)
  updated_by varchar(100)

  Indexes {
    serial_number [unique, name: "uk_instrument_serial"]
    active [name: "idx_calibration_instruments_active"]
    expire_date [name: "idx_calibration_instruments_expire_date"]
  }
}
```

**CHECK:** `expire_date > calibration_date` (เมื่อทั้งคู่ไม่ null)

---

### 3.5 `rtu.calibrations`

```dbml
Table rtu.calibrations {
  id uuid [pk, default: `gen_random_uuid()`, note: 'รหัสรายการสอบเทียบ']
  panel_device_id uuid [not null, note: 'อ้างอิงอุปกรณ์ที่ถูกสอบเทียบ']
  instrument_id uuid [not null, note: 'อ้างอิงเครื่องมือที่ใช้สอบเทียบ']
  performed_by varchar(100) [note: 'ผู้ดำเนินการสอบเทียบ (ชื่อ — business field)']
  performed_at timestamptz [not null, note: 'วันและเวลาที่สอบเทียบ']
  result varchar(20) [not null, note: 'PASS | FAIL | ADJUSTED']
  remark text [note: 'หมายเหตุผลการสอบเทียบ']
  created_at timestamptz [not null, default: `now()`]
  updated_at timestamptz [not null, default: `now()`]
  created_by varchar(100)
  updated_by varchar(100)

  Indexes {
    panel_device_id [name: "idx_calibrations_panel_device_id"]
    instrument_id [name: "idx_calibrations_instrument_id"]
    performed_at [name: "idx_calibrations_performed_at"]
    (panel_device_id, performed_at) [name: "idx_calibrations_device_performed_at"]
  }
}
```

**ไม่มี soft delete** — `DELETE` ลบจริง, readings cascade

---

### 3.6 `rtu.calibration_readings`

```dbml
Table rtu.calibration_readings {
  id uuid [pk, default: `gen_random_uuid()`, note: 'รหัสรายการค่าที่วัดได้']
  calibration_id uuid [not null, note: 'อ้างอิงรายการสอบเทียบ']
  sequence smallint [not null, note: 'ลำดับการวัด (unique ต่อใบ)']
  item_label varchar(150) [note: 'ชื่อหัวข้อหรือจุดที่วัด']
  parameter_key varchar(50) [not null, note: 'รหัสพารามิเตอร์ เช่น pressure, flow']
  value numeric [note: 'ค่าที่วัดได้']
  unit varchar(20) [note: 'หน่วย เช่น bar, psi, mA']
  created_at timestamptz [not null, default: `now()`]
  created_by varchar(100)
  updated_by varchar(100)

  Indexes {
    calibration_id [name: "idx_calibration_readings_calibration_id"]
    parameter_key [name: "idx_calibration_readings_parameter_key"]
    (calibration_id, sequence) [unique, name: "uk_calibration_reading_sequence"]
  }
}
```

---

## 4. References (Foreign Keys)

```dbml
Ref: rtu.panels.id < rtu.panel_devices.panel_id          [delete: restrict, update: cascade]
Ref: rtu.device_models.id < rtu.panel_devices.device_model_id [delete: restrict, update: cascade]
Ref: rtu.panel_devices.id < rtu.calibrations.panel_device_id  [delete: restrict, update: cascade]
Ref: rtu.calibration_instruments.id < rtu.calibrations.instrument_id [delete: restrict, update: cascade]
Ref: rtu.calibrations.id < rtu.calibration_readings.calibration_id  [delete: cascade, update: cascade]
```

---

## 5. สิ่งที่ปรับจาก DBML ตั้งต้น (User Template)

| หัวข้อ | Template เดิม | Implementation จริง |
|--------|---------------|---------------------|
| `created_at` / `updated_at` | บางตาราง nullable | **NOT NULL** + trigger ทุกตารางหลัก |
| `device_models` timestamps | ไม่มี | **มี** created/updated |
| `calibration_instruments` timestamps | ไม่มี | **มี** created/updated |
| `calibrations.updated_at` | ไม่มี | **มี** |
| `created_by` / `updated_by` | ไม่มี | **มี** migration 000002 |
| `communication_status` / `health_status` | nullable | **NOT NULL DEFAULT 'UNKNOWN'** + CHECK |
| `result` | ไม่มี CHECK | **CHECK** PASS/FAIL/ADJUSTED |
| `(calibration_id, sequence)` | index ธรรมดา | **UNIQUE** |
| `(panel_id, tag_name)` | ไม่มี | **partial UNIQUE** |
| tag unique scope | — | ต่อ panel ไม่ใช่ global |

---

## 6. Migration History

| Version | File | รายละเอียด |
|---------|------|------------|
| 1 | `000001_init_rtu_schema` | สร้าง schema `rtu` + 6 ตาราง + triggers |
| 2 | `000002_add_audit_columns` | เพิ่ม `created_by`, `updated_by` ทุกตาราง |

---

## 7. ตาราง PM/CM ที่ Design Doc กำหนด (ยังไม่ implement ใน repo นี้)

สำหรับอ้างอิงเมื่อขยายระบบ — รายการจาก Database Design v1.2:

| Domain | Tables |
|--------|--------|
| Auth | `users`, `password_resets`, `refresh_tokens` |
| Master | `rtu`*, `equipment`*, `checklist_items`, `engineer` |
| Work Order | `work_order`, `work_order_history`, `attachment` |
| PM Report | `pm_report`, `checklist_result`, `pm_ground_test`, `report_image`, `pm_calibration`, `pm_calibration_point`, `pm_power_test`, `pm_power_test_point` |
| CM | `cm_report`, `cm_image` |
| Approval | `wo_approval` |
| System | `notifications`, `logs` |

\* แทนที่ด้วย `rtu.panels`, `rtu.panel_devices`, `rtu.device_models` ใน service นี้

---

## 8. ไฟล์ที่เกี่ยวข้อง

| ไฟล์ | 用途 |
|------|------|
| [`rtu-calibration-schema.dbml`](./rtu-calibration-schema.dbml) | Import dbdiagram.io / dbdocs |
| [`database-design.md`](./database-design.md) | เอกสารนี้ |
| [`../business-logic-api.md`](../business-logic-api.md) | Business rules ฝั่ง API |
| [`../migrations/`](../migrations/) | DDL จริง (source of truth) |

---

*อัปเดต: สอดคล้อง migration version 2 — scrutinize จาก Design Doc v1.2 + System Design + codebase*
