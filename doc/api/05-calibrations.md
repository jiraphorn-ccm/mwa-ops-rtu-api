# 05 — Calibrations

Prefix: `{api_prefix}/calibrations`, `/calibration-instruments`, nested ใต้ `/panel-devices/{id}/calibrations`

---

## Calibration Instruments (เครื่องมืออ้างอิง)

| Method | Path |
|--------|------|
| `GET`/`POST` | `/calibration-instruments` |
| `GET`/`PATCH`/`DELETE` | `/calibration-instruments/{id}` |

### POST — สรุป field

| Field | บังคับ |
|-------|--------|
| `name` | ✅ |
| `manufacturer`, `model`, `serial_no` | ไม่ |
| `calibration_date`, `expire_date` | ไม่ — **expire ต้องหลัง calibration** |
| `active` | bool |

**List filters:** `active`, `manufacturer`, `equipment_type`, `brand`, `expired`, `expiring_before`

---

## Calibration Events

| Method | Path |
|--------|------|
| `GET`/`POST` | `/calibrations` |
| `GET`/`POST` | `/panel-devices/{device_id}/calibrations` |
| `GET`/`PATCH`/`DELETE` | `/calibrations/{id}` |
| `GET` | `/calibrations/summary` |

### POST — `CalibrationCreateInput`

| Field | บังคับ | เงื่อนไข |
|-------|--------|----------|
| `panel_device_id` | ✅ | จาก body หรือ URL nested |
| `instrument_id` | ✅ | เครื่องมือต้อง active, ไม่หมดอายุ ณ `performed_at` |
| `performed_at` | ✅ | ห้ามล่วงหน้าเกิน 5 นาที → `E300_121` |
| `result` | ✅ | `PASS` / `FAIL` / `ADJUSTED` |
| `performed_by` | ไม่ | |
| `remark` | ไม่ | |
| `work_order_id` | ไม่ | **ผูก PM ได้เฉพาะ SIX_MONTH PM** → `E300_240` |
| `pm_report_id` | ไม่ | คู่กับ work order PM 6 เดือน |
| `channel_type` | ไม่ | `PRESSURE`,`FLOW`,`LEVEL`,`RTU_READBACK` |
| `eut_*` | ไม่ | ข้อมูล EUT บนใบ |
| `result_type` | ไม่ | `TESTED`,`CALIBRATED_AND_TESTED`,`OTHER` |
| `readings[]` | ไม่ | ส่ง inline ได้ max 500 แถว |

#### `readings[]`

| Field | บังคับ |
|-------|--------|
| `parameter_key` | ✅ |
| `sequence` | ไม่ — ไม่ส่งจะ auto 1,2,3… |
| `item_label`, `value`, `unit` | ไม่ |

**อุปกรณ์:** ต้อง `active=true` → มิฉะนั้น `E300_111`

---

## Readings (measurement sheet)

| Method | Path | Semantics |
|--------|------|-----------|
| `GET` | `/calibrations/{id}/readings` | List |
| `POST` | `/calibrations/{id}/readings` | เพิ่มแถว |
| `PUT` | `/calibrations/{id}/readings` | **Replace ทั้ง sheet** |
| `GET`/`PATCH`/`DELETE` | `/calibrations/{id}/readings/{readingId}` | ทีละแถว |
| `GET`/`PATCH`/`DELETE` | `/calibration-readings/{id}` | by reading id |

**PUT replace:** ส่ง `readings[]` ครบ — ลบของเดิมแล้วเขียนใหม่ใน transaction

---

## PM 6 เดือน

- Submit PM (`SIX_MONTH`) ต้องมี calibration ≥ 1 ผูกใบ → `E300_237`
- ส่ง `work_order_id` + `pm_report_id` ตอน create calibration เพื่อผูกกับ PM visit

---

## Attachments

| Method | Path |
|--------|------|
| `GET`/`POST` | `/calibrations/{id}/attachments` |
