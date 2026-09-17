# 11 — Dashboard (ภาพรวมระบบ RTU)

Prefix: `{api_prefix}/dashboard`

หน้า dashboard ของ frontend (`cilent/src/features/dashboard`) ใช้ชุดนี้แทนข้อมูลจำลอง  
แผนที่สถานีใช้ `GET /dashboard/map` (ไม่ต้องยิง `GET /panels?limit=100`)

ทุกเส้นต้องมี Bearer — ดู [00-conventions.md](./00-conventions.md)

| Method | Path | ใช้เมื่อ |
|--------|------|----------|
| `GET` | `/dashboard` | ตัวชี้วัด + กราฟ + SLA + เรื่องที่ต้องตัดสินใจ + สถานีเสี่ยง (top 10) |
| `GET` | `/dashboard/stations-at-risk` | ตารางสถานีที่ต้องให้ความสนใจทั้งชุด |
| `GET` | `/dashboard/map` | จุดพิกัดสถานีที่เปิดใช้งาน สำหรับแผนที่ |

Success code: `S201_009` (overview) · `S201_001` (list)

---

## Query

### `GET /dashboard`

| Param | ค่า | Default | ความหมาย |
|-------|------|---------|----------|
| `period` | `TODAY` `7D` `MONTH` `YEAR` | `MONTH` | ปุ่มช่วงเวลาบนหัวหน้า dashboard |

ปฏิทินใช้โซน **`Asia/Bangkok`**

| period | จาก | ถึง | เทียบช่วงก่อนหน้า | ความละเอียดกราฟ |
|--------|-----|-----|-------------------|-----------------|
| `TODAY` | เที่ยงคืนวันนี้ | ตอนนี้ | เมื่อวานช่วงเดียวกัน | รายชั่วโมง |
| `7D` | เที่ยงคืนของ 6 วันก่อน | ตอนนี้ | 7 วันก่อนหน้า | รายวัน |
| `MONTH` | วันที่ 1 ของเดือน | ตอนนี้ | เดือนปฏิทินก่อน (วัน/เวลาเดียวกัน) | รายวัน |
| `YEAR` | 1 ม.ค. ปีนี้ | ตอนนี้ | ปีก่อนช่วงเดียวกัน | รายเดือน |

ค่าที่ไม่ใช่ enum → `400` `E100_004`

### `GET /dashboard/stations-at-risk`

| Param | Default | ช่วง | ความหมาย |
|-------|---------|------|----------|
| `limit` | 50 | 1–200 | จำนวนสถานีสูงสุด |

ไม่ใช่ paginated list — ไม่มี `page` / `meta`

---

## `GET /dashboard` — field ครบ

```json
{
  "generated_at": "2026-09-17T06:35:00.000Z",
  "period": {
    "key": "MONTH",
    "timezone": "Asia/Bangkok",
    "from": "2026-09-01T00:00:00+07:00",
    "to": "2026-09-17T13:35:00+07:00",
    "previous_from": "2026-08-01T00:00:00+07:00",
    "previous_to": "2026-08-17T13:35:00+07:00"
  },
  "targets": {
    "availability_percent": 97,
    "pm_plan_percent": 90,
    "sla_percent": 90,
    "approval_sla_hours": 8
  },
  "metrics": {
    "stations": {
      "total": 132,
      "area_count": 18,
      "online_count": 128,
      "online_percent": 97.0,
      "abnormal_count": 9,
      "critical_count": 2,
      "watch_count": 7
    },
    "pm": {
      "planned": 46,
      "completed_on_plan": 42,
      "percent": 91.3,
      "delta_percent": 4.6
    },
    "cm_open": {
      "count": 17,
      "over_sla": 4,
      "delta_count": 2
    },
    "pending_approvals": {
      "count": 6,
      "over_sla": 2,
      "oldest_age_hours": 19.0
    }
  },
  "availability": {
    "current_percent": 97.0,
    "target_percent": 97,
    "series": [
      { "bucket": "2026-09-01", "label": "1 ก.ย.", "availability": 96.8 }
    ]
  },
  "maintenance": {
    "series": [
      {
        "bucket": "2026-09-01",
        "label": "1 ก.ย.",
        "pm_completed": 2,
        "pm_delayed": 0,
        "cm_completed": 1,
        "cm_open": 4
      }
    ]
  },
  "sla": {
    "percent": 88.0,
    "target_percent": 90,
    "delta_percent": 3.4,
    "within": 64,
    "near": 5,
    "over": 4,
    "avg_close_hours": 11.6
  },
  "decisions": [
    {
      "key": "critical_cm_unstarted",
      "title": "งาน CM ระดับวิกฤตยังไม่มีผู้รับผิดชอบ",
      "detail": "2 งาน · ควรมอบหมายภายในวันนี้",
      "tone": "rose",
      "count": 2
    }
  ],
  "stations_at_risk": [
    {
      "id": "4bccb3c4-899d-439a-9423-f782f8ba4f52",
      "code": "RTU-BRK-02",
      "location": "สถานีบางรัก",
      "operational_status": "ABNORMAL",
      "latest_issue": "ระบบสื่อสารขัดข้อง",
      "open_work_orders": 3,
      "downtime_seconds": 15120
    }
  ]
}
```

`decisions` และ `series` เป็น array ว่างได้ แต่ไม่เป็น `null`

---

## Map ไปที่ UI

| บล็อกบนหน้า | Field |
|-------------|--------|
| สถานี RTU ทั้งหมด | `metrics.stations.total` · `area_count` |
| สถานีออนไลน์ | `online_count` / `online_percent` |
| สถานีผิดปกติ | `abnormal_count` · วิกฤต `critical_count` · เฝ้าระวัง `watch_count` |
| PM สำเร็จตามแผน | `metrics.pm.percent` · `completed_on_plan` จาก `planned` · `delta_percent` |
| งาน CM คงค้าง | `metrics.cm_open.count` · `over_sla` · `delta_count` |
| รายการรออนุมัติ | `metrics.pending_approvals.*` |
| กราฟความพร้อมใช้งาน | `availability.series[]` · เป้าหมาย `targets.availability_percent` |
| เรื่องที่ต้องตัดสินใจ | `decisions[]` |
| กราฟ PM / CM | `maintenance.series[]` |
| การปฏิบัติตาม SLA | `sla.*` |
| สถานีที่ต้องให้ความสนใจ | `stations_at_risk[]` (top 10) — ปุ่มดูทั้งหมด → `GET /dashboard/stations-at-risk` |
| แผนที่ | `GET /dashboard/map` |

`operational_status` ของสถานี: `NORMAL` · `MONITORING` (เฝ้าระวัง) · `ABNORMAL` (วิกฤต) — สูตรเดียวกับ `GET /panels`

---

## นิยามตัวเลข

### สถานี (snapshot ตอนเรียก — ไม่กรองตาม `period`)

นับเฉพาะ `panels.active = true`

| Field | นิยาม |
|-------|--------|
| `online_count` | `operational_status = NORMAL` |
| `critical_count` | `ABNORMAL` (อุปกรณ์ CRITICAL หรือ OFFLINE) |
| `watch_count` | `MONITORING` (WARNING / DEGRADED / UNKNOWN) |
| `abnormal_count` | critical + watch |
| `online_percent` | `online_count / total * 100` ทศนิยม 1 ตำแหน่ง |
| `area_count` | จำนวน `location` ที่ไม่ซ้ำ (ตัดช่องว่าง) |

ลำดับความสำคัญของสถานะตู้: **MONITORING > ABNORMAL > NORMAL** (ตู้ที่มีทั้ง WARNING และ CRITICAL จะเป็น MONITORING)

### PM สำเร็จตามแผน (`period`)

ประชากร: ใบ PM ที่ `active` ไม่ยกเลิก และวันที่แผนอยู่ในช่วง (`planned_date` ถ้ามี ไม่งั้น `due_date` ไม่งั้น `created_at`)

`completed_on_plan` = สถานะ `COMPLETED` / `CONDITIONAL` และ (`due_date` ว่าง หรือวันปิดงาน ≤ due)

`delta_percent` = เปอร์เซ็นต์ช่วงนี้ ลบช่วงก่อนหน้า

### CM คงค้าง

ใบ CM ที่ยังไม่ปิด ณ เวลา `period.to` (สร้างก่อนเวลานั้น และ `closed_at` ว่างหรือยังไม่ถึง)

`over_sla` = มี `due_date` และ due ก่อนวันปฏิทินของเวลานั้น

`delta_count` = จำนวนเปิดตอนนี้ ลบจำนวนที่เปิดอยู่ ณ `previous_to`

### รออนุมัติ

`work_orders.status = PENDING_APPROVAL`

เกิน SLA เมื่อ `current_round.submitted_at` เก่ากว่า **8 ชั่วโมง** (`targets.approval_sla_hours`)

### ความพร้อมใช้งาน (กราฟ)

- จุดล่าสุดของ `series` = `metrics.stations.online_percent` (telemetry จริง)
- จุดย้อนหลัง = proxy จากสัดส่วนตู้ที่**ไม่มี CM เปิดคาบเกี่ยวกับถังเวลา**  
  `100 * (1 - distinct_panel_with_open_CM / total_stations)`  
  ยังไม่มีตารางประวัติ telemetry รายชั่วโมง/วัน

### ผลงาน PM / CM (`maintenance.series`)

| Field | นับอย่างไรในแต่ละถัง |
|-------|----------------------|
| `pm_completed` | PM ปิดงานในถัง และตรงตาม due |
| `pm_delayed` | PM ปิดช้าในถัง หรือยังไม่ปิดแต่ due อยู่ในถังและเลยกำหนด |
| `cm_completed` | CM ปิดในถัง |
| `cm_open` | CM ที่ยังเปิดอยู่ ณ ปลายถัง |

### SLA

ประชากร: ใบงานที่มี `due_date` ไม่ยกเลิก และ (ปิดในช่วง **หรือ** due ในช่วง **หรือ** ยังเปิดและ due ไม่เกินปลายช่วง)

| กลุ่ม | นิยาม |
|-------|--------|
| `within` | ปิดทัน due หรือยังไม่ปิดและ due ≥ วันนี้ + 2 วัน |
| `near` | ยังไม่ปิด และ due เป็นวันนี้หรือวันถัดไป (`near` = 2 วัน) |
| `over` | ปิดช้า หรือยังไม่ปิดและ due ผ่านแล้ว |
| `percent` | `within / (within+near+over) * 100` |
| `avg_close_hours` | ค่าเฉลี่ย `closed_at - created_at` ของใบที่ปิดในช่วง |

### เรื่องที่ต้องตัดสินใจ

คืนเฉพาะรายการที่เข้าเงื่อนไข (array ว่าง = ไม่มีเรื่องเร่ง)

| `key` | เงื่อนไข | `tone` |
|-------|----------|--------|
| `critical_cm_unstarted` | CM `priority=HIGH` สถานะ `ASSIGNED` (ยังไม่ check-in) | `rose` |
| `approvals_over_sla` | รออนุมัติเกิน 8 ชม. | `amber` |
| `pm_below_target` | มี PM ในช่วง และ `%` < 90 | `blue` |

ใบงานทุกใบมีผู้รับผิดชอบตอนสร้าง (`work_order_rounds.assigned_to` NOT NULL) — “ยังไม่มีผู้รับผิดชอบ” ในที่นี้หมายถึง **ยังไม่เริ่มงาน** (สถานะ ASSIGNED)

### สถานีเสี่ยง

ตู้ `active` ที่สถานะ `ABNORMAL` หรือ `MONITORING` เรียง วิกฤตก่อน → งานคงค้างมาก → downtime นาน

| Field | ที่มา |
|-------|--------|
| `latest_issue` | หัวข้อปัญหา (`problem_topics.name`) ของ CM เปิดล่าสุด ถ้าไม่มีใช้ `title` |
| `open_work_orders` | ใบ PM/CM ที่สถานะเปิด |
| `downtime_seconds` | วินาทีนับจาก `min(last_seen_at)` ของอุปกรณ์ CRITICAL/OFFLINE ถ้าไม่มีใช้เวลาสร้าง CM เปิดล่าสุด — เป็น `0` ถ้าไม่มีทั้งคู่ |

---

## `GET /dashboard/map`

ไม่กรอง `period` — snapshot ตู้ที่ `active` และมี `latitude` + `longitude`

```json
{
  "items": [
    {
      "id": "4bccb3c4-899d-439a-9423-f782f8ba4f52",
      "code": "RTU-BRK-02",
      "location": "สถานีบางรัก",
      "latitude": 13.7210000,
      "longitude": 100.5230000,
      "operational_status": "ABNORMAL"
    }
  ]
}
```

Frontend ใช้ `id` `code` `location` `latitude` `longitude` `operational_status` เหมือน `GET /panels` แต่ได้ทั้งชุด ไม่ติด `limit` ของ list ตู้

---

## `GET /dashboard/stations-at-risk`

โครง `data.items[]` เดียวกับ `stations_at_risk` ใน overview — ใช้ปุ่ม “ดูสถานีทั้งหมด”
