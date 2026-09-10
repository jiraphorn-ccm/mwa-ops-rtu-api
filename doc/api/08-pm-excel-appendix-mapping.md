# 08 — PM Excel ผนวก → API Mapping

เอกสารนี้ map **แบบฟอร์ม Excel รายงาน PM** (เช่น `เอกสารรายงาน/รายงาน PM-U120-ครั้งที่4.xlsx`) ไปยัง **field และ `parameter_key`** ของ API

อ้างอิง API:
- [05-calibrations.md](./05-calibrations.md) — **ผนวก 5.1 / 5.2 ฉบับเต็ม** (header map + JSON 30/20 readings copy-paste)
- [02-pm-reports.md](./02-pm-reports.md) — ผนวก 7 (`ground_test`)

> เอกสารนี้สรุป flow PM รวม 3 ผนวก — รายละเอียด calibration ดูที่ **05**

---

## สรุป map

| ผนวก Excel | เนื้อหา | API | `channel_type` / object |
|------------|---------|-----|-------------------------|
| **5.1** | สอบเทียบ Pressure Transmitter | `POST /calibrations` | `PRESSURE` |
| **5.2** | RTU Readback (Generate Input) | `POST /calibrations` | `RTU_READBACK` |
| **7** | วัดกราวด์ L-G / N-G | `PUT /work-orders/{id}/pm-report` | `ground_test` |

**PM 6 เดือน:** ผนวก 5.1 และ 5.2 ต้องส่ง `work_order_id` + `pm_report_id` ตอน create calibration และต้องมี calibration ≥ 1 ก่อน `POST .../pm-report/submit` → `E300_237`

---

## กฎ `readings[]` (ผนวก 5.1 / 5.2)

- แต่ละแถวใน DB = **1 ค่า** (`parameter_key` + `value` + `unit`)
- **`sequence` ห้ามซ้ำ** ในใบ calibration เดียว (unique ต่อ `calibration_id`)
- 1 จุดทดสอบใน Excel = **หลาย readings** (ไม่ใช่ 1 sequence ต่อแถว Excel)

### สูตร sequence

**ผนวก 5.1** — 6 ค่าต่อจุด, 5 จุด = 30 readings:

```
base = (pointNo - 1) × 6 + 1
base+0 → input_mmh2o
base+1 → desired_output_ma
base+2 → as_found_inc
base+3 → as_found_dec
base+4 → as_left_inc
base+5 → as_left_dec
```

**ผนวก 5.2** — 4 ค่าต่อจุด, 5 จุด = 20 readings:

```
base = (pointNo - 1) × 4 + 1
base+0 → generate_input_ma
base+1 → current_pressure_m
base+2 → current_flow_forward
base+3 → current_flow_reverse
```

---

## ผนวก 5.1 — Pressure Transmitter

**Sheet:** `ผนวก5.1`  
**อุปกรณ์ (TAG):** Pressure Transmitter  
**Endpoint:** `POST {api_prefix}/calibrations` หรือ `POST .../panel-devices/{device_id}/calibrations`

### Header Excel → API

| บรรทัด / ช่องใน Excel | API field | ตัวอย่าง (U120 ครั้งที่ 4) |
|------------------------|-----------|----------------------------|
| ชื่ออุปกรณ์ (TAG) | `panel_device_id` | อุปกรณ์ Pressure Transmitter ในตู้ |
| Model/Type (EUT) | `eut_model` | `STX` |
| Serial No (EUT) | `eut_serial_no` | `STX221538` |
| Input Range | `eut_input_range` | `0-25,000 mmH2O` |
| Acc/Class | `eut_accuracy_class` | `Pressure Transmitter` |
| Power Supply | `eut_power_supply` | `24 VDC` |
| Output Range | `eut_output_range` | `4-20mA` |
| เครื่องมือ (คอลัมน์ขวา) | `instrument_id` | FUJI FKGT03V5… / SN A0C1892C |
| วันที่/เวลา | `performed_at` | ISO datetime |
| Result: Tested | `result_type` | `TESTED` |
| Result โดยรวม | `result` | `PASS` / `FAIL` / `ADJUSTED` |
| PM 6 เดือน | `work_order_id`, `pm_report_id` | UUID ใบ PM |

```json
"channel_type": "PRESSURE"
```

### ตาราง Test Equipment → `readings[]`

| คอลัมน์ Excel | `parameter_key` | `unit` |
|--------------|-----------------|--------|
| Input (mmH2O) | `input_mmh2o` | `mmH2O` |
| Desired output (mA) | `desired_output_ma` | `mA` |
| As Found — Inc | `as_found_inc` | `mmH2O` |
| As Found — Dec | `as_found_dec` | `mmH2O` |
| As Left — Inc | `as_left_inc` | `mmH2O` |
| As Left — Dec | `as_left_dec` | `mmH2O` |

### ข้อมูลตัวอย่าง (U120 ครั้งที่ 4)

| จุด | input_mmh2o | desired mA | found Inc | found Dec | left Inc | left Dec |
|-----|-------------|------------|-----------|-----------|----------|----------|
| 1 | 0.027 | 4 | 5 | 0 | 0.027 | 0.027 |
| 2 | 6250 | 8 | 6302 | 6302 | 6250 | 6250 |
| 3 | 12500 | 12 | 12535 | 12536 | 12500 | 12500 |
| 4 | 18750 | 16 | 18777 | 18776 | 18750 | 18750 |
| 5 | 25000 | 20 | 25053 | 25054 | 25000 | 25000 |

### ตัวอย่าง POST body (ครบ 30 readings)

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

## ผนวก 5.2 — RTU Readback (Generate Input)

**Sheet:** `ผนวก5.2`  
**อุปกรณ์ (TAG):** Remote Terminal Unit  
**Endpoint:** `POST {api_prefix}/calibrations`

### Header Excel → API

| บรรทัด / ช่องใน Excel | API field | ตัวอย่าง (U120 ครั้งที่ 4) |
|------------------------|-----------|----------------------------|
| ชื่ออุปกรณ์ (TAG) | `panel_device_id` | อุปกรณ์ RTU ในตู้ |
| Model/Type (EUT) | `eut_model` | `AC500` |
| Serial No (EUT) | `eut_serial_no` | `1S1202000016871` |
| Span Flow / Span Pressure | `eut_input_range` หรือ `remark` | `Flow ±5000 m3/h, Pressure 0-25 m` |
| Acc/Class, Power, Output | `eut_accuracy_class`, `eut_power_supply`, `eut_output_range` | ตามใบ |
| เครื่อง Loop Test (ขวา) | `instrument_id` | DRUCK UPS-II / SN 48787 |
| วันที่/เวลา | `performed_at` | ISO datetime |

```json
"channel_type": "RTU_READBACK"
```

### ตาราง Test Equipment → `readings[]`

คอลัมน์ Analog Input (Pressure / Flow Fwd / Flow Rev) ใน Excel ใช้ค่า mA **เท่ากันต่อแถว** — เก็บ **`generate_input_ma` ครั้งเดียว** ต่อจุด

| คอลัมน์ Excel | `parameter_key` | `unit` |
|--------------|-----------------|--------|
| Generate Input (mA) | `generate_input_ma` | `mA` |
| Current Value — Pressure | `current_pressure_m` | `m` |
| Current Value — Flow Forward | `current_flow_forward` | `m3/h` |
| Current Value — Flow Reverse | `current_flow_reverse` | `m3/h` |

### ข้อมูลตัวอย่าง (U120 ครั้งที่ 4)

| mA | Pressure (m) | Flow Forward | Flow Reverse |
|----|--------------|--------------|--------------|
| 4 | 0.00 | 0 | -10 |
| 8 | 6.39 | 1268 | -1283 |
| 12 | 12.79 | 2551 | -2551 |
| 16 | 19.11 | 3800 | -3800 |
| 20 | 25.1 | 5000 | -5010 |

### ตัวอย่าง POST body (ครบ 20 readings)

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
  "eut_input_range": "Flow Forward/Reverse Max ±5000 m3/h, Pressure 0-25 m",
  "eut_accuracy_class": "Flow Transmitter",
  "eut_power_supply": "24 VDC",
  "eut_output_range": "4-20mA",
  "readings": [
    { "sequence": 1,  "item_label": "4 mA",  "parameter_key": "generate_input_ma",      "value": 4,    "unit": "mA" },
    { "sequence": 2,  "item_label": "4 mA",  "parameter_key": "current_pressure_m",     "value": 0,    "unit": "m" },
    { "sequence": 3,  "item_label": "4 mA",  "parameter_key": "current_flow_forward",   "value": 0,    "unit": "m3/h" },
    { "sequence": 4,  "item_label": "4 mA",  "parameter_key": "current_flow_reverse",   "value": -10,  "unit": "m3/h" },

    { "sequence": 5,  "item_label": "8 mA",  "parameter_key": "generate_input_ma",      "value": 8,    "unit": "mA" },
    { "sequence": 6,  "item_label": "8 mA",  "parameter_key": "current_pressure_m",     "value": 6.39, "unit": "m" },
    { "sequence": 7,  "item_label": "8 mA",  "parameter_key": "current_flow_forward",   "value": 1268, "unit": "m3/h" },
    { "sequence": 8,  "item_label": "8 mA",  "parameter_key": "current_flow_reverse",   "value": -1283,"unit": "m3/h" },

    { "sequence": 9,  "item_label": "12 mA", "parameter_key": "generate_input_ma",      "value": 12,   "unit": "mA" },
    { "sequence": 10, "item_label": "12 mA", "parameter_key": "current_pressure_m",     "value": 12.79,"unit": "m" },
    { "sequence": 11, "item_label": "12 mA", "parameter_key": "current_flow_forward",   "value": 2551, "unit": "m3/h" },
    { "sequence": 12, "item_label": "12 mA", "parameter_key": "current_flow_reverse",   "value": -2551,"unit": "m3/h" },

    { "sequence": 13, "item_label": "16 mA", "parameter_key": "generate_input_ma",      "value": 16,   "unit": "mA" },
    { "sequence": 14, "item_label": "16 mA", "parameter_key": "current_pressure_m",     "value": 19.11,"unit": "m" },
    { "sequence": 15, "item_label": "16 mA", "parameter_key": "current_flow_forward",   "value": 3800, "unit": "m3/h" },
    { "sequence": 16, "item_label": "16 mA", "parameter_key": "current_flow_reverse",   "value": -3800,"unit": "m3/h" },

    { "sequence": 17, "item_label": "20 mA", "parameter_key": "generate_input_ma",      "value": 20,   "unit": "mA" },
    { "sequence": 18, "item_label": "20 mA", "parameter_key": "current_pressure_m",     "value": 25.1, "unit": "m" },
    { "sequence": 19, "item_label": "20 mA", "parameter_key": "current_flow_forward",   "value": 5000, "unit": "m3/h" },
    { "sequence": 20, "item_label": "20 mA", "parameter_key": "current_flow_reverse",   "value": -5010,"unit": "m3/h" }
  ]
}
```

---

## ผนวก 7 — Ground Test (วัดกราวด์)

**Sheet:** `ผนวก7`  
**ไม่ใช่ calibration** — บันทึกใน **PM report aggregate**

**Endpoint:** `PUT {api_prefix}/work-orders/{work_order_id}/pm-report`  
**Object:** `ground_test` (ดู [02-pm-reports.md § ground_test](./02-pm-reports.md))

### ตาราง Excel → API

| ช่อง Excel | API field | หมายเหตุ |
|------------|-----------|----------|
| ความต้านทาน L-G (Ω) | `resistance_lg` | ค่า `OL` (Open Loop) → ส่ง `null` + บันทึกใน `note` |
| ความต้านทาน N-G (Ω) | `resistance_ng` | เช่นเดียวกัน |
| ความต่างศักย์ L-G (V) | `voltage_lg` | decimal |
| ความต่างศักย์ N-G (V) | `voltage_ng` | decimal |
| ผลรวม | `result` | `PASS` / `FAIL` |
| วันที่/เวลา | `measured_at` | ISO datetime |

### ข้อมูลตัวอย่าง (U120 ครั้งที่ 4)

| ช่อง | ค่า |
|------|-----|
| L-G Ω | OL |
| N-G Ω | OL |
| L-G V | 229.7 |
| N-G V | 2.776 |

### ตัวอย่างใน PM report body

`PUT .../pm-report` เป็น **replace aggregate** — ส่ง field ครบทุกครั้ง (รวม `checklist_results`, `power_test` ฯลฯ)

```json
{
  "engineer_id": "{{engineer_id}}",
  "report_date": "2026-08-30T13:06:00+07:00",
  "checklist_results": [],
  "ground_test": {
    "resistance_lg": null,
    "resistance_ng": null,
    "voltage_lg": 229.7,
    "voltage_ng": 2.776,
    "result": "PASS",
    "note": "Resistance L-G: OL, N-G: OL",
    "measured_at": "2026-08-30T13:06:00+07:00"
  },
  "power_test": null
}
```

---

## Flow PM 6 เดือน (รวม 3 ผนวก)

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

## `parameter_key` มาตรฐาน (อ้างอิง schema)

| `channel_type` | keys ที่ใช้ |
|----------------|-------------|
| `PRESSURE`, `FLOW`, `LEVEL` | `input_mmh2o`, `desired_output_ma`, `as_found_inc`, `as_found_dec`, `as_left_inc`, `as_left_dec` |
| `RTU_READBACK` | `generate_input_ma`, `current_pressure_m`, `current_flow_forward`, `current_flow_reverse` |

ดู schema เพิ่ม: `doc/rtu-full-schema.dbml` → `rtu.calibration_readings.parameter_key`

---

## แก้ readings หลังบันทึก

| งาน | Path |
|-----|------|
| แทนที่ sheet ทั้งใบ | `PUT /calibrations/{id}/readings` |
| ดูค่าที่บันทึก | `GET /calibrations/{id}` |
