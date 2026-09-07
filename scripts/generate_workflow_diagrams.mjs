#!/usr/bin/env node
/** Generate draw.io workflow diagrams for RTU PM/CM domain. */
import { writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const OUT = join(
  dirname(fileURLToPath(import.meta.url)),
  "..",
  "doc",
  "diagrams",
  "rtu-pm-cm-workflows.drawio",
);

const S = {
  title:
    "text;html=1;strokeColor=none;fillColor=none;align=center;verticalAlign=middle;fontSize=20;fontStyle=1",
  subtitle:
    "text;html=1;strokeColor=none;fillColor=#FFF2CC;align=center;verticalAlign=middle;fontSize=11;rounded=1;",
  pm: "rounded=1;whiteSpace=wrap;html=1;fillColor=#DAE8FC;strokeColor=#6C8EBF;align=left;spacingLeft=8;",
  cm: "rounded=1;whiteSpace=wrap;html=1;fillColor=#F8CECC;strokeColor=#B85450;align=left;spacingLeft=8;",
  ok: "rounded=1;whiteSpace=wrap;html=1;fillColor=#D5E8D4;strokeColor=#82B366;align=left;spacingLeft=8;",
  warn: "rounded=1;whiteSpace=wrap;html=1;fillColor=#FFE6CC;strokeColor=#D79B00;align=left;spacingLeft=8;",
  decision:
    "rhombus;whiteSpace=wrap;html=1;fillColor=#FFE6CC;strokeColor=#D79B00;fontStyle=1",
  note: "shape=note;whiteSpace=wrap;html=1;size=14;fillColor=#FFFFCC;strokeColor=#CCCC00;align=left;spacingLeft=8;",
  state:
    "ellipse;whiteSpace=wrap;html=1;fillColor=#E1D5E7;strokeColor=#9673A6;fontStyle=1",
  end: "ellipse;whiteSpace=wrap;html=1;fillColor=#D5E8D4;strokeColor=#82B366;fontStyle=1",
  lane:
    "swimlane;horizontal=0;whiteSpace=wrap;html=1;fillColor=#F5F5F5;strokeColor=#666666;fontStyle=1;startSize=28;",
};

let cellId = 0;
const nextId = () => `c${++cellId}`;

function cell(value, style, x, y, w, h, id = nextId()) {
  return { id, value, style, x, y, w, h };
}

function edge(source, target, label = "", dashed = false) {
  const id = nextId();
  return {
    id,
    source,
    target,
    label,
    dashed,
    style: `edgeStyle=orthogonalEdgeStyle;rounded=0;html=1;${
      dashed ? "dashed=1;" : ""
    }`,
  };
}

function renderPage(name, nodes, edges, pageH = 1200) {
  cellId = 0;
  const nodeMap = new Map(nodes.map((n) => [n.id, n]));
  let xml = "";
  for (const n of nodes) {
    xml += `        <mxCell id="${n.id}" value="${esc(n.value)}" style="${n.style}" vertex="1" parent="1">
          <mxGeometry x="${n.x}" y="${n.y}" width="${n.w}" height="${n.h}" as="geometry" />
        </mxCell>\n`;
  }
  for (const e of edges) {
    xml += `        <mxCell id="${e.id}" value="${esc(e.label)}" style="${e.style}" edge="1" parent="1" source="${e.source}" target="${e.target}">
          <mxGeometry relative="1" as="geometry" />
        </mxCell>\n`;
  }
  void nodeMap;
  return `  <diagram id="${slug(name)}" name="${escAttr(name)}">
    <mxGraphModel dx="1400" dy="900" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1400" pageHeight="${pageH}" math="0" shadow="0">
      <root>
        <mxCell id="0" />
        <mxCell id="1" parent="0" />
${xml}      </root>
    </mxGraphModel>
  </diagram>`;
}

function esc(s) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/\n/g, "&#xa;");
}

function escAttr(s) {
  return s.replace(/"/g, "&quot;");
}

function slug(s) {
  return s.replace(/[^a-zA-Z0-9]+/g, "-").toLowerCase();
}

function pageOverview() {
  const n = [
    cell(
      "RTU PM/CM — ภาพรวมงานทั้งหมด (Work Types)",
      S.title,
      280,
      20,
      840,
      36,
    ),
    cell(
      "แหล่งความจริง: doc/api/*.md · internal/service · Postman 02 — PM Smoke Flow",
      S.subtitle,
      280,
      58,
      840,
      28,
    ),
    cell(
      "🔵 PM Work Order&#xa;POST /work-orders&#xa;work_order_type=PM&#xa;+ pm_schedule_type&#xa;&#xa;→ lifecycle แยก (ดู tab 01)&#xa;→ มี work_order_no PM-...",
      S.pm,
      80,
      120,
      280,
      130,
    ),
    cell(
      "🔴 CM Work Order (Standalone)&#xa;POST /work-orders&#xa;work_order_type=CM&#xa;+ problem_topic_id(s)&#xa;&#xa;→ duplicate check panel+topic&#xa;→ lifecycle แยก (ดู tab 02)",
      S.cm,
      400,
      120,
      280,
      130,
    ),
    cell(
      "🟢 Onsite Fix (ไม่มี CM WO)&#xa;POST /pm-reports/{id}/onsite-fixes&#xa;&#xa;cm_reports PM_ONSITE_FIX&#xa;work_order_id = NULL&#xa;จบด้วย ended_at",
      S.ok,
      720,
      120,
      280,
      130,
    ),
    cell(
      "🟠 Escalate ระหว่าง PM&#xa;POST /pm-reports/{id}/escalate&#xa;&#xa;spawn CM WO ใหม่&#xa;cm_reports PM_ESCALATED&#xa;CM duplicate → 409",
      S.warn,
      80,
      290,
      280,
      120,
    ),
    cell(
      "🟣 Approval Escalate&#xa;POST /work-orders/{id}/approvals&#xa;decision=REJECTED + escalate&#xa;&#xa;หลัง submit PM แล้ว&#xa;reuse CM เปิด topic เดียวกันได้",
      S.warn,
      400,
      290,
      280,
      120,
    ),
    cell(
      "⚪ Shared Workflow Engine&#xa;work_orders + work_order_rounds&#xa;status เปลี่ยนผ่าน action API เท่านั้น&#xa;PATCH ห้ามส่ง status&#xa;(ดู tab 03 State Machine)",
      S.note,
      720,
      290,
      280,
      120,
    ),
    cell(
      "สิ่งที่ไม่ใช่ Work Order&#xa;• cm_reports PM_ONSITE_FIX — sub-record ใต้ PM&#xa;• calibrations — โดเมนแยก (ผูก PM 6 เดือนตอน submit)&#xa;• attachments / notifications — cross-cutting",
      S.note,
      80,
      450,
      920,
      80,
    ),
    cell(
      "อ่านต่อ&#xa;Tab 01 PM lifecycle · Tab 02 CM lifecycle · Tab 03 Status · Tab 04 Repair during PM · Tab 05 CM origins · Tab 06 Approval",
      S.subtitle,
      80,
      550,
      920,
      40,
    ),
  ];
  return renderPage("00 — Overview", n, [], 650);
}

function pagePmLifecycle() {
  const a = nextId();
  const b = nextId();
  const c = nextId();
  const d = nextId();
  const e = nextId();
  const f = nextId();
  const g = nextId();
  const h = nextId();
  const i = nextId();
  const j = nextId();
  const k = nextId();
  const l = nextId();
  const m = nextId();
  const n = nextId();
  const o = nextId();
  const p = nextId();
  const q = nextId();

  const nodes = [
    {
      id: a,
      value: "01 — PM Work Order Lifecycle (เต็มวงจร)",
      style: S.title,
      x: 300,
      y: 20,
      w: 800,
      h: 36,
    },
    {
      id: b,
      value:
        "POST /work-orders&#xa;work_order_type=PM · pm_schedule_type=THREE_MONTH|SIX_MONTH&#xa;requested_by · assigned_to · assigned_by · panel_id&#xa;→ 201 · work_order_no PM-{panel}-{seq} · status=ASSIGNED · round_no=1",
      style: S.pm,
      x: 420,
      y: 70,
      w: 360,
      h: 90,
    },
    {
      id: c,
      value:
        "Optional: POST .../reassign&#xa;(ก่อน check-in เท่านั้น · E300_208 ถ้า check-in แล้ว)",
      style: S.note,
      x: 80,
      y: 100,
      w: 300,
      h: 50,
    },
    {
      id: d,
      value: "POST .../check-in&#xa;status → IN_PROGRESS",
      style: S.pm,
      x: 420,
      y: 180,
      w: 360,
      h: 50,
    },
    {
      id: e,
      value:
        "PUT .../pm-report (replace aggregate · DRAFT)&#xa;engineer_id · checklist_results[] · ground_test · power_test",
      style: S.pm,
      x: 420,
      y: 250,
      w: 360,
      h: 70,
    },
    {
      id: f,
      value:
        "ระหว่างทำ PM: แจ้งซ่อม?&#xa;(ดู tab 04 — onsite / escalate)",
      style: S.decision,
      x: 470,
      y: 340,
      w: 260,
      h: 80,
    },
    {
      id: g,
      value:
        "Optional: POST .../check-out&#xa;บันทึกเวลาออก · status ยัง IN_PROGRESS",
      style: S.note,
      x: 80,
      y: 350,
      w: 300,
      h: 50,
    },
    {
      id: h,
      value:
        "POST .../pm-report/submit · actor_id&#xa;status → PENDING_APPROVAL",
      style: S.pm,
      x: 420,
      y: 440,
      w: 360,
      h: 55,
    },
    {
      id: i,
      value:
        "Submit validation&#xa;THREE_MONTH → ต้องมี power_test (E300_236)&#xa;SIX_MONTH → calibration ≥1 ผูก PM (E300_237)",
      style: S.warn,
      x: 820,
      y: 430,
      w: 280,
      h: 75,
    },
    {
      id: j,
      value:
        "POST .../approvals · reviewer_id · decision&#xa;APPROVED | APPROVED_CONDITION | REJECTED",
      style: S.pm,
      x: 420,
      y: 520,
      w: 360,
      h: 60,
    },
    {
      id: k,
      value:
        "APPROVED / APPROVED_CONDITION&#xa;status → COMPLETED | CONDITIONAL&#xa;sync panel PM dates",
      style: S.ok,
      x: 120,
      y: 610,
      w: 280,
      h: 70,
    },
    {
      id: l,
      value:
        "REJECTED rework&#xa;status → PENDING · round ใหม่&#xa;reassign_to (optional)",
      style: S.warn,
      x: 460,
      y: 610,
      w: 280,
      h: 70,
    },
    {
      id: m,
      value:
        "REJECTED + escalate=true&#xa;PM → CONDITIONAL · spawn/reuse CM&#xa;(ดู tab 06)",
      style: S.warn,
      x: 800,
      y: 610,
      w: 280,
      h: 70,
    },
    {
      id: n,
      value: "PENDING → check-in รอบใหม่ → IN_PROGRESS (loop)",
      style: S.note,
      x: 460,
      y: 700,
      w: 280,
      h: 45,
    },
    {
      id: o,
      value: "Terminal: COMPLETED · CONDITIONAL · CANCELLED",
      style: S.end,
      x: 420,
      y: 780,
      w: 360,
      h: 45,
    },
  ];

  return renderPage("01 — PM Work Order", nodes, [
    edge(b, d),
    edge(d, e),
    edge(e, f),
    edge(f, h, "ทำ PM ต่อ"),
    edge(h, j),
    edge(j, k, "APPROVED"),
    edge(j, l, "REJECTED rework"),
    edge(j, m, "REJECTED escalate"),
    edge(l, n),
    edge(n, d, "round+1", true),
    edge(k, o),
  ], 880);
}

function pageCmLifecycle() {
  const nodes = [
    cell(
      "02 — CM Work Order Lifecycle (Standalone / STANDALONE)",
      S.title,
      280,
      20,
      840,
      36,
    ),
    cell(
      "POST /work-orders · work_order_type=CM&#xa;problem_topic_id หรือ problem_topic_ids[] (≥1)&#xa;server seed cm_reports + work_order_problem_topics ใน tx เดียว",
      S.cm,
      400,
      70,
      400,
      80,
    ),
    cell(
      "Duplicate guard&#xa;panel + topic ซ้ำ CM เปิดอยู่&#xa;status ∈ ASSIGNED, IN_PROGRESS, PENDING, PENDING_APPROVAL&#xa;→ 409 E300_246",
      S.warn,
      80,
      70,
      280,
      90,
    ),
    cell("status = ASSIGNED · work_order_no CM-...", S.cm, 400, 170, 400, 40),
    cell(
      "POST .../reassign (ก่อน check-in)&#xa;POST .../check-in → IN_PROGRESS",
      S.cm,
      400,
      230,
      400,
      50,
    ),
    cell(
      "PUT .../cm-report (replace · ส่งครบทุกครั้ง)&#xa;problem_topic_id / problem_topic_ids&#xa;sync add-only topics → junction",
      S.cm,
      400,
      300,
      400,
      70,
    ),
    cell(
      "PATCH /work-orders/{id}&#xa;problem_topic_ids = replace ชุด topic ทั้งใบ (CM only)",
      S.note,
      80,
      300,
      280,
      55,
    ),
    cell(
      "POST .../cm-report/submit · actor_id&#xa;→ PENDING_APPROVAL",
      S.cm,
      400,
      390,
      400,
      50,
    ),
    cell(
      "POST .../approvals&#xa;APPROVED → COMPLETED&#xa;REJECTED → PENDING + round ใหม่",
      S.cm,
      400,
      460,
      400,
      60,
    ),
    cell(
      "Multi-topic: duplicate ตรวจทีละ topic&#xa;GET .../open-cm-work-orders — UI เตือน CM เปิดบนตู้",
      S.note,
      80,
      460,
      280,
      60,
    ),
    cell("Terminal · COMPLETED / rework loop", S.end, 400, 540, 400, 40),
  ];
  const ids = nodes.map((n) => n.id);
  return renderPage(
    "02 — CM Work Order",
    nodes,
    [
      edge(ids[1], ids[3]),
      edge(ids[3], ids[4]),
      edge(ids[4], ids[5]),
      edge(ids[5], ids[7]),
      edge(ids[7], ids[8]),
      edge(ids[8], ids[10]),
    ],
    620,
  );
}

function pageStatusMachine() {
  const nodes = [
    cell("03 — Work Order Status State Machine", S.title, 320, 20, 760, 36),
    cell(
      "status เปลี่ยนผ่าน action endpoint เท่านั้น — ห้าม PATCH status&#xa;ไม่มี status REJECTED บน work_orders (REJECTED = decision ใน approvals)",
      S.subtitle,
      200,
      58,
      1000,
      36,
    ),
    cell("ASSIGNED", S.state, 120, 140, 140, 50),
    cell("IN_PROGRESS", S.state, 340, 140, 140, 50),
    cell("PENDING_APPROVAL", S.state, 560, 140, 160, 50),
    cell("COMPLETED", S.end, 800, 120, 120, 50),
    cell("CONDITIONAL", S.end, 800, 190, 120, 50),
    cell("PENDING", S.state, 340, 280, 140, 50),
    cell("CANCELLED", S.end, 120, 280, 120, 50),
    cell("check-in", S.note, 250, 125, 70, 30),
    cell("submit report", S.note, 490, 125, 90, 30),
    cell("APPROVED", S.note, 720, 110, 70, 30),
    cell("APPROVED_CONDITION", S.note, 720, 180, 120, 30),
    cell("REJECTED rework", S.note, 520, 265, 100, 30),
    cell("check-out ไม่เปลี่ยน status", S.note, 340, 200, 160, 30),
  ];
  const [a, b, c, d, e, f, g] = nodes.map((n) => n.id);
  return renderPage(
    "03 — Status State Machine",
    nodes,
    [
      edge(a, b, "check-in"),
      edge(b, c, "submit"),
      edge(c, d, "APPROVED"),
      edge(c, e, "APPROVED_CONDITION"),
      edge(c, g, "REJECTED"),
      edge(g, b, "check-in round N+1", true),
    ],
    400,
  );
}

function pageRepairDuringPm() {
  // Reuse structure from existing diagram — simplified ids
  const nodes = [
    cell("04 — แจ้งซ่อมระหว่างทำ PM", S.title, 320, 20, 760, 36),
    cell(
      "Precondition: PM WO · check-in แล้ว · มี pm_report_id (PUT pm-report สร้าง draft ได้)",
      S.subtitle,
      200,
      58,
      1000,
      32,
    ),
    cell("เจอปัญหาระหว่าง PM?", S.decision, 480, 110, 200, 90),
    cell(
      "ไม่เจอ → ทำ checklist ต่อ → submit PM",
      S.ok,
      820,
      120,
      240,
      50,
    ),
    cell("ซ่อมหน้างานได้? (จบในวัน)", S.decision, 480, 230, 200, 90),
    cell(
      "✅ POST .../onsite-fixes&#xa;PM_ONSITE_FIX · ไม่มี CM WO · ไม่มี work_order_no&#xa;ended_at default now = จบแล้ว",
      S.ok,
      80,
      360,
      320,
      90,
    ),
    cell(
      "❌ POST .../pm-reports/{id}/escalate&#xa;spawn CM WO · PM_ESCALATED · CM เริ่ม PENDING&#xa;duplicate topic → 409 (ไม่ reuse)",
      S.cm,
      760,
      360,
      320,
      90,
    ),
    cell("ทำ PM ต่อได้ · CM แยก workflow", S.note, 400, 480, 360, 40),
    cell(
      "เปรียบเทียบ escalate อีกทาง: tab 06 (หลัง submit · approval reject · reuse CM ได้)",
      S.note,
      80,
      540,
      1000,
      40,
    ),
  ];
  const [d1, d2, onsite, esc] = [nodes[2].id, nodes[4].id, nodes[5].id, nodes[6].id];
  return renderPage(
    "04 — Repair During PM",
    nodes,
    [
      edge(d1, nodes[3].id, "ไม่"),
      edge(d1, d2, "เจอ"),
      edge(d2, onsite, "ได้"),
      edge(d2, esc, "ไม่ได้"),
      edge(onsite, nodes[7].id),
      edge(esc, nodes[7].id),
    ],
    620,
  );
}

function pageCmOrigins() {
  const nodes = [
    cell("05 — CM Report 3 Origins (cm_reports)", S.title, 280, 20, 840, 36),
    cell(
      "Origin คำนวณจาก FK — ไม่มี column origin แยก&#xa;CHECK: work_order_id IS NOT NULL OR pm_report_id IS NOT NULL",
      S.subtitle,
      200,
      58,
      1000,
      36,
    ),
    cell(
      "STANDALONE&#xa;work_order_id ✓&#xa;pm_report_id ✗&#xa;&#xa;POST /work-orders CM&#xa;มี CM workflow เต็ม · มี work_order_no",
      S.cm,
      80,
      120,
      280,
      120,
    ),
    cell(
      "PM_ONSITE_FIX&#xa;work_order_id ✗&#xa;pm_report_id ✓&#xa;&#xa;POST .../onsite-fixes&#xa;ไม่มี CM WO · จบด้วย ended_at",
      S.ok,
      400,
      120,
      280,
      120,
    ),
    cell(
      "PM_ESCALATED&#xa;work_order_id ✓&#xa;pm_report_id ✓&#xa;&#xa;POST .../escalate หรือ approval escalate&#xa;CM WO แยก + ผูก PM report",
      S.warn,
      720,
      120,
      280,
      120,
    ),
    cell(
      "สถานะ CM&#xa;• มี work_order_id → ดู work_orders.status&#xa;• PM_ONSITE_FIX → ไม่มี status column · completed โดยนัย&#xa;• ไม่ปรากฏใน open-cm-work-orders (onsite)",
      S.note,
      80,
      280,
      920,
      70,
    ),
    cell(
      "Multi-topic (CM WO): work_order_problem_topics junction&#xa;cm_reports.problem_topic_id = topic หลักของรอบ · report PUT/PATCH sync add-only",
      S.note,
      80,
      370,
      920,
      50,
    ),
  ];
  return renderPage("05 — CM Report Origins", nodes, [], 460);
}

function pageApproval() {
  const nodes = [
    cell("06 — Approval · Reject · Escalate", S.title, 320, 20, 760, 36),
    cell(
      "POST /work-orders/{id}/approvals · WO status ต้อง = PENDING_APPROVAL",
      S.subtitle,
      240,
      58,
      920,
      28,
    ),
    cell("decision?", S.decision, 500, 110, 180, 80),
    cell(
      "APPROVED&#xa;→ COMPLETED",
      S.ok,
      120,
      220,
      200,
      50,
    ),
    cell(
      "APPROVED_CONDITION&#xa;→ CONDITIONAL",
      S.ok,
      120,
      290,
      200,
      50,
    ),
    cell("REJECTED", S.warn, 500, 220, 180, 50),
    cell("escalate?", S.decision, 500, 300, 180, 70),
    cell(
      "Rework&#xa;status → PENDING&#xa;round ใหม่ · reassign_to optional&#xa;→ check-in ทำใหม่",
      S.pm,
      760,
      220,
      260,
      80,
    ),
    cell(
      "Escalate CM&#xa;PM → CONDITIONAL&#xa;repair_date + problem_topic_id บังคับ&#xa;reuse CM เปิด topic เดียวกันได้",
      S.cm,
      760,
      320,
      260,
      90,
    ),
    cell(
      "ต่างจาก POST /pm-reports/.../escalate&#xa;• escalate หน้างาน: 409 ถ้า duplicate · ไม่ reuse&#xa;• approval escalate: reuse CM ได้ · หลัง submit PM แล้ว",
      S.note,
      80,
      430,
      940,
      60,
    ),
  ];
  const [d1, rej, d2, rework, esc] = [
    nodes[2].id,
    nodes[5].id,
    nodes[6].id,
    nodes[7].id,
    nodes[8].id,
  ];
  return renderPage(
    "06 — Approval",
    nodes,
    [
      edge(d1, nodes[3].id, "APPROVED"),
      edge(d1, nodes[4].id, "APPROVED_CONDITION"),
      edge(d1, rej, "REJECTED"),
      edge(rej, d2),
      edge(d2, rework, "false"),
      edge(d2, esc, "true"),
    ],
    530,
  );
}

const pages = [
  pageOverview(),
  pagePmLifecycle(),
  pageCmLifecycle(),
  pageStatusMachine(),
  pageRepairDuringPm(),
  pageCmOrigins(),
  pageApproval(),
];

const xml = `<mxfile host="app.diagrams.net" modified="${new Date().toISOString()}" agent="RTU API workflow generator" version="22.1.0">
${pages.join("\n")}
</mxfile>
`;

mkdirSync(dirname(OUT), { recursive: true });
writeFileSync(OUT, xml, "utf8");
console.log(`Wrote ${OUT}`);
console.log(`Pages: ${pages.length}`);
