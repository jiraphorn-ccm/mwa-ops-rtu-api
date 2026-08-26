#!/usr/bin/env node
/** Generate doc/rtu_db_dictionary.html from doc/rtu-full-schema.dbml */
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const DBML = join(root, "doc", "rtu-full-schema.dbml");
const OUT = join(root, "doc", "rtu_db_dictionary.html");

const MODULES = [
  { id: "all", label: "ทั้งหมด" },
  { id: "core", label: "Core RTU" },
  { id: "calibration", label: "สอบเทียบ" },
  { id: "pm_cm", label: "PM / CM" },
  { id: "files", label: "ไฟล์ / แจ้งเตือน" },
];

const MODULE_TABLES = {
  all: [],
  core: ["panels", "device_models", "panel_devices", "panel_images"],
  calibration: [
    "calibration_instruments",
    "calibrations",
    "calibration_readings",
  ],
  pm_cm: [
    "engineers",
    "checklist_items",
    "work_orders",
    "work_order_rounds",
    "work_order_activity_logs",
    "wo_approvals",
    "pm_reports",
    "checklist_results",
    "pm_ground_tests",
    "pm_power_tests",
    "pm_power_test_points",
    "cm_reports",
  ],
  files: ["attachments", "notifications"],
};

const PG_ENUM = new Set([
  "operational_status",
  "communication_status",
  "health_status",
  "calibration_result",
  "panel_image_type",
  "work_order_type",
  "pm_schedule_type",
  "work_order_status",
  "work_order_priority",
  "work_order_source",
  "work_order_activity_action",
  "wo_approval_decision",
  "pm_report_status",
  "checklist_action_type",
  "applicable_pm",
  "ground_test_result",
  "power_test_equipment_role",
  "power_test_result",
  "calibration_channel_type",
  "calibration_result_type",
  "attachment_entity_type",
  "notification_type",
]);

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function cleanNote(s) {
  return (s ?? "")
    .replace(/\[IMPLEMENTED\]\s*/g, "")
    .replace(/\[PROPOSED\]\s*/g, "")
    .replace(/\[เพิ่มใหม่ - PROPOSED\]\s*/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

function sqlType(name, rawType) {
  const t = rawType.toLowerCase();
  if (PG_ENUM.has(rawType)) return `varchar /* ${rawType} */`;
  if (t === "uuid") return "uuid";
  if (t.startsWith("varchar")) return rawType;
  if (t === "text") return "text";
  if (t === "boolean") return "boolean";
  if (t === "date") return "date";
  if (t.startsWith("timestamptz")) return "timestamptz";
  if (t.startsWith("numeric")) return rawType;
  if (t === "bigint") return "bigint";
  if (t === "smallint") return "smallint";
  if (t === "int") return "integer";
  return rawType;
}

function parseRefs(content) {
  const fks = new Map();
  const re =
    /Ref:\s*rtu\.(\w+)\.(\w+)\s*[-]?\s*[>]?\s*rtu\.(\w+)\.(\w+)/g;
  let m;
  while ((m = re.exec(content))) {
    const [, fromTable, fromCol, toTable, toCol] = m;
    fks.set(`${fromTable}.${fromCol}`, `${toTable}.${toCol}`);
  }
  return fks;
}

function parseTables(content) {
  const tables = [];
  const re = /Table\s+rtu\.(\w+)\s*\{([\s\S]*?)^\}/gm;
  let m;
  while ((m = re.exec(content))) {
    const name = m[1];
    const body = m[2];
    const columns = [];
    let tableNote = "";

    const noteBlock = body.match(/^\s*Note:\s*'''([\s\S]*?)'''/m);
    if (noteBlock) tableNote = cleanNote(noteBlock[1]);

    const lines = body.split("\n");
    for (const line of lines) {
      if (/^\s*(Indexes|Note:)/.test(line)) continue;
      const col = line.match(
        /^\s+(\w+)\s+([A-Za-z0-9_()[\],]+?)(?:\s+\[(.*)\])?\s*$/,
      );
      if (!col) continue;
      const [, colName, rawType, attrsRaw = ""] = col;
      if (colName === "Indexes") break;

      const attrs = attrsRaw.toLowerCase();
      const noteMatch = attrsRaw.match(/note:\s*'((?:\\'|[^'])*)'/);
      const noteMatchMulti = attrsRaw.match(/note:\s*`([^`]*)`/);
      let note = "";
      if (noteMatch) note = noteMatch[1].replace(/\\'/g, "'");
      else if (noteMatchMulti) note = noteMatchMulti[1];

      columns.push({
        name: colName,
        type: sqlType(colName, rawType.trim()),
        pk: attrs.includes("pk"),
        unique: attrs.includes("unique"),
        notNull: attrs.includes("not null") || attrs.includes("pk"),
        note: cleanNote(note),
      });
    }

    tables.push({ name, columns, tableNote });
  }
  return tables;
}

function moduleFor(table) {
  for (const [mod, list] of Object.entries(MODULE_TABLES)) {
    if (mod !== "all" && list.includes(table)) return mod;
  }
  return "core";
}

function renderColumn(col, tableName, fks) {
  const fk = fks.get(`${tableName}.${col.name}`);
  const tags = [];
  if (col.pk) tags.push('<span class="tag pk">PK</span>');
  if (fk) tags.push('<span class="tag fk">FK</span>');
  else if (col.unique) tags.push('<span class="tag uq">UQ</span>');

  const nullMark = col.notNull ? "NOT NULL" : "NULL";
  const ref = fk ? `rtu.${fk}` : "";
  const noteParts = [col.note, !col.pk && !fk && col.unique ? "UNIQUE" : ""]
    .filter(Boolean)
    .join(" · ");

  return `<tr>
        <td class="col-name">${esc(col.name)}</td>
        <td class="col-type">${esc(col.type)}</td>
        <td class="col-key">${tags.join(" ") || "—"}<br><span class="col-null">${nullMark}</span></td>
        <td class="col-ref">${esc(ref)}</td>
        <td class="col-note">${esc(noteParts)}</td>
      </tr>`;
}

function renderTable(t, fks) {
  const mod = moduleFor(t.name);
  const pkCount = t.columns.filter((c) => c.pk).length;
  const fkCount = t.columns.filter((c) => fks.has(`${t.name}.${c.name}`)).length;
  const subtitle = [
    `${t.columns.length} columns`,
    pkCount ? `${pkCount} PK` : null,
    fkCount ? `${fkCount} FK` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  const rows = t.columns.map((c) => renderColumn(c, t.name, fks)).join("\n");
  const tableNote = t.tableNote
    ? `<p class="table-note">${esc(t.tableNote)}</p>`
    : "";

  return `<section class="table-card" id="table-${t.name}" data-module="${mod}">
      <header class="table-card-head">
        <h2><a href="#table-${t.name}">rtu.${esc(t.name)}</a></h2>
        <span>${esc(subtitle)}</span>
      </header>
      ${tableNote}
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Column</th>
              <th>Type</th>
              <th>Key</th>
              <th>Reference</th>
              <th>Note</th>
            </tr>
          </thead>
          <tbody>
${rows}
          </tbody>
        </table>
      </div>
    </section>`;
}

function buildHtml(tables, fks) {
  MODULE_TABLES.all = tables.map((t) => t.name);
  const tableCards = tables.map((t) => renderTable(t, fks)).join("\n");
  const nav = MODULES.map(
    (m) =>
      `<button type="button" class="nav-btn${m.id === "all" ? " active" : ""}" data-module="${m.id}">${esc(m.label)}</button>`,
  ).join("\n");

  const today = new Date().toISOString().slice(0, 10);
  const refCount = fks.size;

  return `<!DOCTYPE html>
<html lang="th">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>RTU Data Dictionary</title>
<style>
    :root {
      --bg: #f4f5f7;
      --paper: #fff;
      --line: #c8cdd3;
      --text: #111;
      --muted: #5f6672;
      --accent: #1a5276;
      --head: #e8eef3;
      --pk: #7c3d0a;
      --fk: #145c45;
      --uq: #1e40af;
    }
    * { box-sizing: border-box; }
    html, body {
      margin: 0;
      height: 100%;
      font: 14px/1.45 "Segoe UI", "Sarabun", sans-serif;
      color: var(--text);
      background: var(--bg);
    }
    body {
      display: grid;
      grid-template-rows: auto auto auto 1fr;
      height: 100vh;
    }
    .page-head {
      background: var(--paper);
      border-bottom: 1px solid var(--line);
      padding: 14px 20px 10px;
    }
    .page-head h1 { margin: 0; font-size: 20px; font-weight: 700; }
    .page-head p { margin: 4px 0 0; color: var(--muted); font-size: 13px; }
    .page-nav {
      display: flex;
      flex-wrap: wrap;
      background: #eceff2;
      border-bottom: 1px solid var(--line);
      padding: 0 8px;
    }
    .nav-btn {
      border: none;
      background: transparent;
      padding: 10px 14px;
      font: inherit;
      font-size: 13px;
      color: var(--muted);
      cursor: pointer;
      border-bottom: 3px solid transparent;
    }
    .nav-btn:hover { color: var(--text); background: rgba(255,255,255,.55); }
    .nav-btn.active {
      color: var(--accent);
      font-weight: 600;
      border-bottom-color: var(--accent);
      background: var(--paper);
    }
    .dict-toolbar {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 10px 16px;
      background: var(--paper);
      border-bottom: 1px solid var(--line);
    }
    .dict-toolbar input {
      flex: 1;
      max-width: 420px;
      padding: 7px 10px;
      border: 1px solid var(--line);
      border-radius: 4px;
      font: inherit;
    }
    .dict-toolbar span { color: var(--muted); font-size: 13px; }
    .dict-list { overflow: auto; padding: 16px 20px 32px; }
    .table-card {
      background: var(--paper);
      border: 1px solid var(--line);
      margin-bottom: 16px;
    }
    .table-card-head {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 14px;
      background: var(--head);
      border-bottom: 1px solid var(--line);
    }
    .table-card-head h2 {
      margin: 0;
      font-size: 15px;
      font-family: Consolas, "Courier New", monospace;
    }
    .table-card-head h2 a { color: inherit; text-decoration: none; }
    .table-card-head h2 a:hover { text-decoration: underline; }
    .table-card-head span { color: var(--muted); font-size: 12px; }
    .table-note {
      margin: 0;
      padding: 8px 14px;
      font-size: 12px;
      color: var(--muted);
      border-bottom: 1px solid #e5e7eb;
      background: #fafbfc;
    }
    .table-wrap { overflow-x: auto; }
    .table-card table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }
    .table-card th, .table-card td {
      padding: 6px 12px;
      border-bottom: 1px solid #e5e7eb;
      text-align: left;
      vertical-align: top;
    }
    .table-card th {
      background: #fafbfc;
      font-size: 12px;
      color: var(--muted);
      font-weight: 600;
    }
    .col-name { font-family: Consolas, "Courier New", monospace; font-weight: 600; }
    .col-type { font-family: Consolas, "Courier New", monospace; color: #374151; white-space: nowrap; }
    .col-key { white-space: nowrap; }
    .col-null { font-size: 11px; color: var(--muted); font-weight: 400; }
    .col-ref { font-family: Consolas, "Courier New", monospace; font-size: 12px; color: var(--fk); }
    .col-note { color: var(--muted); font-size: 12px; }
    .tag {
      display: inline-block;
      padding: 1px 6px;
      border-radius: 3px;
      font-size: 11px;
      font-weight: 700;
      line-height: 1.4;
    }
    .tag.pk { background: #fef3c7; color: var(--pk); }
    .tag.fk { background: #d1fae5; color: var(--fk); }
    .tag.uq { background: #dbeafe; color: var(--uq); }
    .table-card.hidden { display: none; }
</style>
</head>
<body>
  <header class="page-head">
    <h1>RTU Data Dictionary</h1>
    <p>Source: doc/rtu-full-schema.dbml · migrations/000001–000006 · schema <code>rtu</code> · ${tables.length} tables · ${refCount} foreign keys · ${today}</p>
  </header>

  <nav class="page-nav" aria-label="Modules">${nav}</nav>

  <div class="dict-toolbar">
    <input type="search" id="dictSearch" placeholder="ค้นหาตารางหรือคอลัมน์…" autocomplete="off">
    <span id="tableCount">${tables.length} tables</span>
  </div>

  <main class="dict-list">${tableCards}</main>

<script>
    const MODULE_TABLES = ${JSON.stringify(MODULE_TABLES)};
    let activeModule = 'all';

    function applyFilters() {
      const q = (document.getElementById('dictSearch').value || '').trim().toLowerCase();
      const allowed = new Set(MODULE_TABLES[activeModule] || []);
      let visible = 0;

      document.querySelectorAll('.table-card').forEach((card) => {
        const name = card.id.replace('table-', '');
        const inModule = activeModule === 'all' || allowed.has(name);
        const matches = !q || card.textContent.toLowerCase().includes(q);
        const show = inModule && matches;
        card.classList.toggle('hidden', !show);
        if (show) visible++;
      });

      document.getElementById('tableCount').textContent = visible + ' tables';
    }

    document.querySelectorAll('.nav-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        activeModule = btn.dataset.module;
        document.querySelectorAll('.nav-btn').forEach((b) => b.classList.toggle('active', b === btn));
        applyFilters();
      });
    });

    document.getElementById('dictSearch').addEventListener('input', applyFilters);
  </script>
</body>
</html>
`;
}

const content = readFileSync(DBML, "utf8");
const fks = parseRefs(content);
const tables = parseTables(content);
if (tables.length === 0) {
  console.error("No tables parsed from", DBML);
  process.exit(1);
}
writeFileSync(OUT, buildHtml(tables, fks), "utf8");
console.log(`Wrote ${OUT} (${tables.length} tables, ${fks.size} FKs)`);
