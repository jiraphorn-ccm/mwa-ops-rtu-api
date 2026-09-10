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
- ผนวก 7 (วัดกราวด์) **ไม่ใช่ calibration** — บันทึกใน `PUT .../pm-report` → `ground_test` (ดู [02-pm-reports.md](./02-pm-reports.md))

---

## Map Excel ผนวก → API (สรุป)

| ผนวก Excel | เนื้อหา | API | `channel_type` |
|------------|---------|-----|----------------|
| **5.1** | สอบเทียบ Pressure Transmitter | `POST /calibrations` | `PRESSURE` |
| **5.2** | RTU Readback (Generate Input) | `POST /calibrations` | `RTU_READBACK` |
| **7** | วัดกราวด์ L-G / N-G | `PUT .../pm-report` | `ground_test` (ไม่ใช่ calibration) |

**แหล่งข้อมูลตัวอย่าง:** `เอกสารรายงาน/รายงาน PM-U120-ครั้งที่4.xlsx` (sheets `ผนวก5.1`, `ผนวก5.2`, `ผนวก7`)

### กฎ `readings[]`

- แต่ละแถวใน DB = **1 ค่า** (`parameter_key` + `value` + `unit`)
- **`sequence` ห้ามซ้ำ** ในใบ calibration เดียว (unique ต่อ `calibration_id`)
- 1 จุดทดสอบใน Excel = **หลาย readings** (ไม่ใช่ 1 sequence ต่อแถว Excel)

**สูตร sequence — ผนวก 5.1** (6 ค่าต่อจุด, 5 จุด = 30 readings):

```
base = (pointNo - 1) × 6 + 1
base+0 → input_mmh2o
base+1 → desired_output_ma
base+2 → as_found_inc
base+3 → as_found_dec
base+4 → as_left_inc
base+5 → as_left_dec
```

**สูตร sequence — ผนวก 5.2** (4 ค่าต่อจุด, 5 จุด = 20 readings):

```
base = (pointNo - 1) × 4 + 1
base+0 → generate_input_ma
base+1 → current_pressure_m
base+2 → current_flow_forward
base+3 → current_flow_reverse
```

### `parameter_key` มาตรฐาน

| `channel_type` | keys ที่ใช้ |
|----------------|-------------|
| `PRESSURE`, `FLOW`, `LEVEL` | `input_mmh2o`, `desired_output_ma`, `as_found_inc`, `as_found_dec`, `as_left_inc`, `as_left_dec` |
| `RTU_READBACK` | `generate_input_ma`, `current_pressure_m`, `current_flow_forward`, `current_flow_reverse` |

---

## ผนวก 5.1 — Pressure Transmitter (สอบเทียบ Pressure)

**API:** `POST {api_prefix}/calibrations`  
**`channel_type`:** `"PRESSURE"`  
**อุปกรณ์ (TAG):** Pressure Transmitter → ผูก `panel_device_id`

### Header (จาก Excel)

| Excel | API field |
|-------|-----------|
| ชื่ออุปกรณ์: Pressure Transmitter | `panel_device_id` (TAG Pressure Transmitter) |
| Model STX / Serial STX221538 | `eut_model`, `eut_serial_no` |
| Input 0-25,000 mmH2O | `eut_input_range` |
| Acc/Class, Power 24 VDC, Output 4-20mA | `eut_accuracy_class`, `eut_power_supply`, `eut_output_range` |
| เครื่อง FUJI FKGT03V5… / Serial A0C1892C | `instrument_id` |
| วันที่ 30 ส.ค.68 | `performed_at` |
| Result: Tested | `result_type`: `"TESTED"`, `result`: `"PASS"` |

### ตาราง Test Equipment → `readings[]`

**5 จุด × 6 ค่า = 30 readings** (`sequence` 1–30)

| คอลัมน์ Excel | `parameter_key` | `unit` |
|--------------|-----------------|--------|
| Input (mmH2O) | `input_mmh2o` | `mmH2O` |
| Desired output (mA) | `desired_output_ma` | `mA` |
| As Found — Inc | `as_found_inc` | `mmH2O` |
| As Found — Dec | `as_found_dec` | `mmH2O` |
| As Left — Inc | `as_left_inc` | `mmH2O` |
| As Left — Dec | `as_left_dec` | `mmH2O` |

### ข้อมูลจากไฟล์ครั้งที่ 4

| จุด | input | desired | found Inc | found Dec | left Inc | left Dec |
|-----|-------|---------|-----------|-----------|----------|----------|
| 1 | 0.027 | 4 | 5 | 0 | 0.027 | 0.027 |
| 2 | 6250 | 8 | 6302 | 6302 | 6250 | 6250 |
| 3 | 12500 | 12 | 12535 | 12536 | 12500 | 12500 |
| 4 | 18750 | 16 | 18777 | 18776 | 18750 | 18750 |
| 5 | 25000 | 20 | 25053 | 25054 | 25000 | 25000 |

### ตัวอย่าง POST body (ครบ 30 readings — copy-paste ได้)

```json
{
  "panel_device_id": "{{panel_device_id}}",
  "instrument_id": "{{instrument_id}}",
  "channel_type": "PRESSURE",
  "performed_at": "2026-08-30T13:06:00+07:00",
  "performed_by": "Somchai",
  "result": "PASS",
  "result_type": "TESTED",
  "work_order_id": "{{work_order_id}}",
  "pm_report_id": "{{pm_report_id}}",
  "eut_model": "STX",
  "eut_serial_no": "STX221538",
  "eut_input_range": "0-25,000 mmH2O",
  "eut_accuracy_class": "Pressure Transmitter",
  "eut_power_supply": "24 VDC",
  "eut_output_range": "4-20mA",
  "readings": [
    { "sequence": 1,  "item_label": "Point 1", "parameter_key": "input_mmh2o",       "value": 0.027, "unit": "mmH2O" },
    { "sequence": 2,  "item_label": "Point 1", "parameter_key": "desired_output_ma", "value": 4,     "unit": "mA" },
    { "sequence": 3,  "item_label": "Point 1", "parameter_key": "as_found_inc",      "value": 5,     "unit": "mmH2O" },
    { "sequence": 4,  "item_label": "Point 1", "parameter_key": "as_found_dec",      "value": 0,     "unit": "mmH2O" },
    { "sequence": 5,  "item_label": "Point 1", "parameter_key": "as_left_inc",       "value": 0.027, "unit": "mmH2O" },
    { "sequence": 6,  "item_label": "Point 1", "parameter_key": "as_left_dec",       "value": 0.027, "unit": "mmH2O" },

    { "sequence": 7,  "item_label": "Point 2", "parameter_key": "input_mmh2o",       "value": 6250,  "unit": "mmH2O" },
    { "sequence": 8,  "item_label": "Point 2", "parameter_key": "desired_output_ma", "value": 8,     "unit": "mA" },
    { "sequence": 9,  "item_label": "Point 2", "parameter_key": "as_found_inc",      "value": 6302,  "unit": "mmH2O" },
    { "sequence": 10, "item_label": "Point 2", "parameter_key": "as_found_dec",      "value": 6302,  "unit": "mmH2O" },
    { "sequence": 11, "item_label": "Point 2", "parameter_key": "as_left_inc",       "value": 6250,  "unit": "mmH2O" },
    { "sequence": 12, "item_label": "Point 2", "parameter_key": "as_left_dec",       "value": 6250,  "unit": "mmH2O" },

    { "sequence": 13, "item_label": "Point 3", "parameter_key": "input_mmh2o",       "value": 12500, "unit": "mmH2O" },
    { "sequence": 14, "item_label": "Point 3", "parameter_key": "desired_output_ma", "value": 12,    "unit": "mA" },
    { "sequence": 15, "item_label": "Point 3", "parameter_key": "as_found_inc",      "value": 12535, "unit": "mmH2O" },
    { "sequence": 16, "item_label": "Point 3", "parameter_key": "as_found_dec",      "value": 12536, "unit": "mmH2O" },
    { "sequence": 17, "item_label": "Point 3", "parameter_key": "as_left_inc",       "value": 12500, "unit": "mmH2O" },
    { "sequence": 18, "item_label": "Point 3", "parameter_key": "as_left_dec",       "value": 12500, "unit": "mmH2O" },

    { "sequence": 19, "item_label": "Point 4", "parameter_key": "input_mmh2o",       "value": 18750, "unit": "mmH2O" },
    { "sequence": 20, "item_label": "Point 4", "parameter_key": "desired_output_ma", "value": 16,    "unit": "mA" },
    { "sequence": 21, "item_label": "Point 4", "parameter_key": "as_found_inc",      "value": 18777, "unit": "mmH2O" },
    { "sequence": 22, "item_label": "Point 4", "parameter_key": "as_found_dec",      "value": 18776, "unit": "mmH2O" },
    { "sequence": 23, "item_label": "Point 4", "parameter_key": "as_left_inc",       "value": 18750, "unit": "mmH2O" },
    { "sequence": 24, "item_label": "Point 4", "parameter_key": "as_left_dec",       "value": 18750, "unit": "mmH2O" },

    { "sequence": 25, "item_label": "Point 5", "parameter_key": "input_mmh2o",       "value": 25000, "unit": "mmH2O" },
    { "sequence": 26, "item_label": "Point 5", "parameter_key": "desired_output_ma", "value": 20,    "unit": "mA" },
    { "sequence": 27, "item_label": "Point 5", "parameter_key": "as_found_inc",      "value": 25053, "unit": "mmH2O" },
    { "sequence": 28, "item_label": "Point 5", "parameter_key": "as_found_dec",      "value": 25054, "unit": "mmH2O" },
    { "sequence": 29, "item_label": "Point 5", "parameter_key": "as_left_inc",       "value": 25000, "unit": "mmH2O" },
    { "sequence": 30, "item_label": "Point 5", "parameter_key": "as_left_dec",       "value": 25000, "unit": "mmH2O" }
  ]
}
```

**Result checkbox บน Excel**

| ติ๊ก | `result_type` |
|------|---------------|
| Tested | `TESTED` |
| Calibrated and Tested | `CALIBRATED_AND_TESTED` |
| Other | `OTHER` + `result_other_text` |

---

## ผนวก 5.2 — RTU Readback (Loop Test)

**API:** `POST {api_prefix}/calibrations`  
**`channel_type`:** `"RTU_READBACK"`  
**อุปกรณ์ (TAG):** Remote Terminal Unit → ผูก `panel_device_id`

### Header (จาก Excel)

| Excel | API field |
|-------|-----------|
| Model AC500 / Serial 1S1202000016871 | `eut_model`, `eut_serial_no` |
| Span Flow ±5,000 m³/h, Span Pressure 0–25 m | `eut_input_range` (หรือ `remark`) |
| เครื่อง DRUCK UPS-II / Serial 48787 | `instrument_id` |
| Result | sheet ไม่มี checkbox — ใส่ `result`: `"PASS"` |

### ตาราง Generate Input → `readings[]`

**5 จุด × 4 ค่า = 20 readings** (`sequence` 1–20)

| Excel | `parameter_key` | `unit` |
|-------|-----------------|--------|
| AI mA (4–20) | `generate_input_ma` | `mA` |
| Current Pressure | `current_pressure_m` | `m` |
| Current Flow Forward | `current_flow_forward` | `m3/h` |
| Current Flow Reverse | `current_flow_reverse` | `m3/h` |

### ข้อมูลครั้งที่ 4

| mA | Pressure (m) | Flow Fwd | Flow Rev |
|----|--------------|----------|----------|
| 4 | 0.00 | 0 | -10 |
| 8 | 6.39 | 1268 | -1283 |
| 12 | 12.79 | 2551 | -2551 |
| 16 | 19.11 | 3800 | -3800 |
| 20 | 25.1 | 5000 | -5010 |

### ตัวอย่าง POST body (ครบ 20 readings — copy-paste ได้)

```json
{
  "panel_device_id": "{{panel_device_id}}",
  "instrument_id": "{{instrument_id}}",
  "channel_type": "RTU_READBACK",
  "performed_at": "2026-08-30T13:06:00+07:00",
  "performed_by": "Somchai",
  "result": "PASS",
  "work_order_id": "{{work_order_id}}",
  "pm_report_id": "{{pm_report_id}}",
  "eut_model": "AC500",
  "eut_serial_no": "1S1202000016871",
  "eut_input_range": "Flow ±5000 m3/h, Pressure 0-25 m",
  "eut_power_supply": "24 VDC",
  "eut_output_range": "4-20mA",
  "readings": [
    { "sequence": 1,  "item_label": "4 mA",  "parameter_key": "generate_input_ma",    "value": 4,    "unit": "mA" },
    { "sequence": 2,  "item_label": "4 mA",  "parameter_key": "current_pressure_m",   "value": 0,    "unit": "m" },
    { "sequence": 3,  "item_label": "4 mA",  "parameter_key": "current_flow_forward", "value": 0,    "unit": "m3/h" },
    { "sequence": 4,  "item_label": "4 mA",  "parameter_key": "current_flow_reverse", "value": -10,  "unit": "m3/h" },

    { "sequence": 5,  "item_label": "8 mA",  "parameter_key": "generate_input_ma",    "value": 8,    "unit": "mA" },
    { "sequence": 6,  "item_label": "8 mA",  "parameter_key": "current_pressure_m",   "value": 6.39, "unit": "m" },
    { "sequence": 7,  "item_label": "8 mA",  "parameter_key": "current_flow_forward", "value": 1268, "unit": "m3/h" },
    { "sequence": 8,  "item_label": "8 mA",  "parameter_key": "current_flow_reverse", "value": -1283,"unit": "m3/h" },

    { "sequence": 9,  "item_label": "12 mA", "parameter_key": "generate_input_ma",    "value": 12,   "unit": "mA" },
    { "sequence": 10, "item_label": "12 mA", "parameter_key": "current_pressure_m",   "value": 12.79,"unit": "m" },
    { "sequence": 11, "item_label": "12 mA", "parameter_key": "current_flow_forward", "value": 2551, "unit": "m3/h" },
    { "sequence": 12, "item_label": "12 mA", "parameter_key": "current_flow_reverse", "value": -2551,"unit": "m3/h" },

    { "sequence": 13, "item_label": "16 mA", "parameter_key": "generate_input_ma",    "value": 16,   "unit": "mA" },
    { "sequence": 14, "item_label": "16 mA", "parameter_key": "current_pressure_m",   "value": 19.11,"unit": "m" },
    { "sequence": 15, "item_label": "16 mA", "parameter_key": "current_flow_forward", "value": 3800, "unit": "m3/h" },
    { "sequence": 16, "item_label": "16 mA", "parameter_key": "current_flow_reverse", "value": -3800,"unit": "m3/h" },

    { "sequence": 17, "item_label": "20 mA", "parameter_key": "generate_input_ma",    "value": 20,   "unit": "mA" },
    { "sequence": 18, "item_label": "20 mA", "parameter_key": "current_pressure_m",   "value": 25.1, "unit": "m" },
    { "sequence": 19, "item_label": "20 mA", "parameter_key": "current_flow_forward", "value": 5000, "unit": "m3/h" },
    { "sequence": 20, "item_label": "20 mA", "parameter_key": "current_flow_reverse", "value": -5010,"unit": "m3/h" }
  ]
}
```

---

## ผนวก 7 — Ground Test (อ้างอิง)

**ไม่ใช่ calibration** — บันทึกใน `PUT {api_prefix}/work-orders/{work_order_id}/pm-report` → object `ground_test`

| Excel | API field |
|-------|-----------|
| L-G ความต้านทาน (Ω) = OL | `resistance_lg` → `null` + ใส่ `"OL"` ใน `note` |
| N-G ความต้านทาน (Ω) = OL | `resistance_ng` → `null` + `note` |
| L-G ความต่างศักย์ (V) = 229.7 | `voltage_lg`: `229.7` |
| N-G ความต่างศักย์ (V) = 2.776 | `voltage_ng`: `2.776` |

```json
{
  "ground_test": {
    "resistance_lg": null,
    "resistance_ng": null,
    "voltage_lg": 229.7,
    "voltage_ng": 2.776,
    "result": "PASS",
    "note": "Resistance L-G: OL, N-G: OL",
    "measured_at": "2026-08-30T13:06:00+07:00"
  }
}
```

รายละเอียด PM report aggregate: [02-pm-reports.md](./02-pm-reports.md) · flow รวม 3 ผนวก: [08-pm-excel-appendix-mapping.md](./08-pm-excel-appendix-mapping.md)

---

## Flow PM 6 เดือน (รวม calibration)

```
1. POST /work-orders                    (pm_schedule_type: SIX_MONTH)
2. PUT  /work-orders/{id}/pm-report     (draft + ground_test ผนวก 7)
3. POST /calibrations                   (ผนวก 5.1 — PRESSURE)
4. POST /calibrations                   (ผนวก 5.2 — RTU_READBACK)
5. PUT  /work-orders/{id}/pm-report     (อัปเดต aggregate ครบถ้วน)
6. POST /work-orders/{id}/pm-report/submit
7. POST /work-orders/{id}/approvals
```

---

## Attachments

| Method | Path |
|--------|------|
| `GET`/`POST` | `/calibrations/{id}/attachments` |
