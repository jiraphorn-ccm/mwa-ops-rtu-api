-- PM/CM report domain: work orders (with multi-round rework tracking),
-- approvals, activity log, PM report + checklist/ground test/power test,
-- CM report, polymorphic attachments, notifications, and master data
-- (engineers, checklist_items). Also extends rtu.calibrations and
-- rtu.panels with the columns the PM flow needs.
--
-- Design reference: doc/rtu-full-schema.dbml, doc/rtu_db_dictionary.html
--
-- Enums are stored as varchar + CHECK, matching the convention set by
-- migration 000001 (no native PostgreSQL ENUM type is used anywhere).

-- ---------------------------------------------------------------------------
-- engineers (master data — printed on PM reports, not an auth user)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.engineers (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name  varchar(255) NOT NULL,
    license_no varchar(100),
    position   varchar(255),
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid
);

CREATE INDEX idx_engineers_active ON rtu.engineers (active);

CREATE TRIGGER trg_engineers_updated_at
    BEFORE UPDATE ON rtu.engineers
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- checklist_items (master data — the standard 13-item PM checklist)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.checklist_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(20) NOT NULL,
    name        varchar(255) NOT NULL,
    action_type varchar(20) NOT NULL,
    applicable_pm varchar(10) NOT NULL DEFAULT 'BOTH',
    sort_order  smallint NOT NULL,
    active      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    created_by  uuid,
    updated_by  uuid,
    CONSTRAINT uk_checklist_items_code UNIQUE (code),
    CONSTRAINT ck_checklist_items_action_type
        CHECK (action_type IN ('MAINTENANCE', 'MEASUREMENT', 'VISUAL_INSPECTION')),
    CONSTRAINT ck_checklist_items_applicable_pm
        CHECK (applicable_pm IN ('PM3', 'PM6', 'BOTH'))
);

CREATE INDEX idx_checklist_items_sort_order ON rtu.checklist_items (sort_order);

CREATE TRIGGER trg_checklist_items_updated_at
    BEFORE UPDATE ON rtu.checklist_items
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- work_orders (PM + CM share one table, distinguished by work_order_type)
-- current_round_id references work_order_rounds, which references this table
-- back — added as a deferred ALTER TABLE below once that table exists.
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.work_orders (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_no    varchar(100) NOT NULL,

    work_order_type  varchar(10) NOT NULL,
    pm_schedule_type varchar(20),

    panel_id         uuid NOT NULL,
    panel_device_id  uuid,

    title            varchar(255),
    description      text,

    status           varchar(20) NOT NULL DEFAULT 'ASSIGNED',
    priority         varchar(10) NOT NULL DEFAULT 'MEDIUM',
    source           varchar(20) NOT NULL DEFAULT 'WORKFLOW',

    requested_by     uuid NOT NULL,
    current_round_id uuid,

    related_work_order_id uuid,

    planned_date     date,
    due_date         date,

    closed_at        timestamptz,
    active           boolean NOT NULL DEFAULT true,

    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    created_by       uuid,
    updated_by       uuid,

    CONSTRAINT uk_work_orders_no UNIQUE (work_order_no),
    CONSTRAINT fk_work_orders_panel FOREIGN KEY (panel_id)
        REFERENCES rtu.panels (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_work_orders_panel_device FOREIGN KEY (panel_device_id)
        REFERENCES rtu.panel_devices (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_work_orders_related FOREIGN KEY (related_work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT ck_work_orders_type CHECK (work_order_type IN ('PM', 'CM')),
    CONSTRAINT ck_work_orders_pm_schedule_type
        CHECK (pm_schedule_type IS NULL OR pm_schedule_type IN ('THREE_MONTH', 'SIX_MONTH')),
    CONSTRAINT ck_work_orders_pm_schedule CHECK (
        (work_order_type = 'PM' AND pm_schedule_type IS NOT NULL)
        OR (work_order_type = 'CM' AND pm_schedule_type IS NULL)
    ),
    CONSTRAINT ck_work_orders_status CHECK (status IN
        ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL', 'COMPLETED', 'CONDITIONAL', 'CANCELLED')),
    CONSTRAINT ck_work_orders_priority CHECK (priority IN ('HIGH', 'MEDIUM', 'LOW')),
    CONSTRAINT ck_work_orders_source CHECK (source IN ('WORKFLOW', 'LEGACY_IMPORT'))
);

CREATE INDEX idx_work_orders_type_status ON rtu.work_orders (work_order_type, status);
CREATE INDEX idx_work_orders_panel_id ON rtu.work_orders (panel_id);
CREATE INDEX idx_work_orders_panel_device_id ON rtu.work_orders (panel_device_id);
CREATE INDEX idx_work_orders_related ON rtu.work_orders (related_work_order_id);
CREATE INDEX idx_work_orders_current_round ON rtu.work_orders (current_round_id);
CREATE INDEX idx_work_orders_requested_by ON rtu.work_orders (requested_by);
CREATE INDEX idx_work_orders_planned_date ON rtu.work_orders (planned_date);
CREATE INDEX idx_work_orders_status_due ON rtu.work_orders (status, due_date);

CREATE TRIGGER trg_work_orders_updated_at
    BEFORE UPDATE ON rtu.work_orders
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- work_order_rounds (every visit/rework attempt of a work order)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.work_order_rounds (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_id uuid NOT NULL,
    round_no      smallint NOT NULL,

    assigned_to   uuid NOT NULL,
    assigned_by   uuid NOT NULL,
    assigned_at   timestamptz NOT NULL,

    check_in_at   timestamptz,
    check_in_lat  numeric(10, 7),
    check_in_lng  numeric(10, 7),
    check_out_at  timestamptz,
    check_out_lat numeric(10, 7),
    check_out_lng numeric(10, 7),

    submitted_at  timestamptz,

    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    created_by    uuid,
    updated_by    uuid,

    CONSTRAINT fk_wo_rounds_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT uk_wo_rounds_wo_round_no UNIQUE (work_order_id, round_no),
    CONSTRAINT ck_wo_rounds_round_no CHECK (round_no > 0),
    CONSTRAINT ck_wo_rounds_check_in_lat CHECK (check_in_lat IS NULL OR check_in_lat BETWEEN -90 AND 90),
    CONSTRAINT ck_wo_rounds_check_in_lng CHECK (check_in_lng IS NULL OR check_in_lng BETWEEN -180 AND 180),
    CONSTRAINT ck_wo_rounds_check_out_lat CHECK (check_out_lat IS NULL OR check_out_lat BETWEEN -90 AND 90),
    CONSTRAINT ck_wo_rounds_check_out_lng CHECK (check_out_lng IS NULL OR check_out_lng BETWEEN -180 AND 180)
);

CREATE INDEX idx_wo_rounds_wo_id ON rtu.work_order_rounds (work_order_id);
CREATE INDEX idx_wo_rounds_assigned_to ON rtu.work_order_rounds (assigned_to);

CREATE TRIGGER trg_wo_rounds_updated_at
    BEFORE UPDATE ON rtu.work_order_rounds
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- Deferred FK closing the work_orders <-> work_order_rounds cycle.
ALTER TABLE rtu.work_orders
    ADD CONSTRAINT fk_work_orders_current_round FOREIGN KEY (current_round_id)
        REFERENCES rtu.work_order_rounds (id) ON UPDATE CASCADE ON DELETE SET NULL;

-- ---------------------------------------------------------------------------
-- work_order_activity_logs (immutable audit trail — who did what, when)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.work_order_activity_logs (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_id        uuid NOT NULL,
    work_order_round_id  uuid,

    action        varchar(20) NOT NULL,
    from_status   varchar(20),
    to_status     varchar(20),
    from_assignee uuid,
    to_assignee   uuid,
    note          text,

    actor_id   uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_wo_activity_logs_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_wo_activity_logs_round FOREIGN KEY (work_order_round_id)
        REFERENCES rtu.work_order_rounds (id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT ck_wo_activity_logs_action CHECK (action IN
        ('ASSIGNED', 'REASSIGNED', 'CHECKED_IN', 'CHECKED_OUT', 'SUBMITTED',
         'STATUS_CHANGED', 'APPROVED', 'APPROVED_COND', 'REJECTED', 'CANCELLED', 'CM_SPAWNED')),
    CONSTRAINT ck_wo_activity_logs_from_status CHECK (from_status IS NULL OR from_status IN
        ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL', 'COMPLETED', 'CONDITIONAL', 'CANCELLED')),
    CONSTRAINT ck_wo_activity_logs_to_status CHECK (to_status IS NULL OR to_status IN
        ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL', 'COMPLETED', 'CONDITIONAL', 'CANCELLED'))
);

CREATE INDEX idx_wo_activity_logs_wo_id ON rtu.work_order_activity_logs (work_order_id);
CREATE INDEX idx_wo_activity_logs_wo_time ON rtu.work_order_activity_logs (work_order_id, created_at);
CREATE INDEX idx_wo_activity_logs_round_id ON rtu.work_order_activity_logs (work_order_round_id);
CREATE INDEX idx_wo_activity_logs_actor ON rtu.work_order_activity_logs (actor_id);
CREATE INDEX idx_wo_activity_logs_action ON rtu.work_order_activity_logs (action);

-- ---------------------------------------------------------------------------
-- wo_approvals (single source of truth for approve/reject decisions)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.wo_approvals (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_id       uuid NOT NULL,
    work_order_round_id uuid NOT NULL,

    reviewer_id   uuid NOT NULL,
    decision      varchar(20) NOT NULL,
    repair_date   date,
    new_work_order_id uuid,
    note          text,

    reviewed_at timestamptz NOT NULL DEFAULT now(),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    created_by  uuid,
    updated_by  uuid,

    CONSTRAINT fk_wo_approvals_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_wo_approvals_round FOREIGN KEY (work_order_round_id)
        REFERENCES rtu.work_order_rounds (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_wo_approvals_new_work_order FOREIGN KEY (new_work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uk_wo_approvals_round_id UNIQUE (work_order_round_id),
    CONSTRAINT ck_wo_approvals_decision
        CHECK (decision IN ('APPROVED', 'APPROVED_CONDITION', 'REJECTED'))
);

CREATE INDEX idx_wo_approvals_wo_id ON rtu.wo_approvals (work_order_id);
CREATE INDEX idx_wo_approvals_new_wo_id ON rtu.wo_approvals (new_work_order_id);
CREATE INDEX idx_wo_approvals_reviewer_id ON rtu.wo_approvals (reviewer_id);

CREATE TRIGGER trg_wo_approvals_updated_at
    BEFORE UPDATE ON rtu.wo_approvals
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- pm_reports
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.pm_reports (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_id       uuid NOT NULL,
    work_order_round_id uuid NOT NULL,
    panel_id            uuid NOT NULL,
    engineer_id         uuid,

    submitted_by uuid,
    status       varchar(20) NOT NULL DEFAULT 'DRAFT',
    note         text,

    report_date  timestamptz,
    submitted_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_pm_reports_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_pm_reports_round FOREIGN KEY (work_order_round_id)
        REFERENCES rtu.work_order_rounds (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_pm_reports_panel FOREIGN KEY (panel_id)
        REFERENCES rtu.panels (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_pm_reports_engineer FOREIGN KEY (engineer_id)
        REFERENCES rtu.engineers (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uk_pm_reports_round_id UNIQUE (work_order_round_id),
    CONSTRAINT ck_pm_reports_status CHECK (status IN ('DRAFT', 'SUBMITTED'))
);

CREATE INDEX idx_pm_reports_work_order_id ON rtu.pm_reports (work_order_id);
CREATE INDEX idx_pm_reports_panel_id ON rtu.pm_reports (panel_id);
CREATE INDEX idx_pm_reports_engineer_id ON rtu.pm_reports (engineer_id);

CREATE TRIGGER trg_pm_reports_updated_at
    BEFORE UPDATE ON rtu.pm_reports
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- checklist_results
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.checklist_results (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pm_report_id       uuid NOT NULL,
    checklist_item_id  uuid NOT NULL,
    panel_device_id    uuid,

    status   varchar(20),
    value    varchar(255),
    meter_no varchar(50),
    note     text,

    checked_by uuid,
    checked_at timestamptz NOT NULL DEFAULT now(),

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_checklist_results_report FOREIGN KEY (pm_report_id)
        REFERENCES rtu.pm_reports (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_checklist_results_item FOREIGN KEY (checklist_item_id)
        REFERENCES rtu.checklist_items (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_checklist_results_panel_device FOREIGN KEY (panel_device_id)
        REFERENCES rtu.panel_devices (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uk_checklist_results_report_item_device
        UNIQUE (pm_report_id, checklist_item_id, panel_device_id)
);

-- Postgres UNIQUE does not de-duplicate NULL panel_device_id (panel-level
-- checklist items), so a partial index covers that case separately.
CREATE UNIQUE INDEX uk_checklist_results_report_item_null_device
    ON rtu.checklist_results (pm_report_id, checklist_item_id) WHERE panel_device_id IS NULL;

CREATE INDEX idx_checklist_results_report_id ON rtu.checklist_results (pm_report_id);
CREATE INDEX idx_checklist_results_item_id ON rtu.checklist_results (checklist_item_id);
CREATE INDEX idx_checklist_results_panel_device_id ON rtu.checklist_results (panel_device_id);

CREATE TRIGGER trg_checklist_results_updated_at
    BEFORE UPDATE ON rtu.checklist_results
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- pm_ground_tests (optional, either PM type)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.pm_ground_tests (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pm_report_id uuid NOT NULL,

    resistance_lg numeric(10, 3),
    resistance_ng numeric(10, 3),
    voltage_lg    numeric(10, 3),
    voltage_ng    numeric(10, 3),
    result        varchar(10),
    note          text,

    measured_by uuid,
    measured_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_pm_ground_tests_report FOREIGN KEY (pm_report_id)
        REFERENCES rtu.pm_reports (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT uk_pm_ground_tests_report_id UNIQUE (pm_report_id),
    CONSTRAINT ck_pm_ground_tests_result CHECK (result IS NULL OR result IN ('PASS', 'FAIL'))
);

CREATE TRIGGER trg_pm_ground_tests_updated_at
    BEFORE UPDATE ON rtu.pm_ground_tests
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- pm_power_tests + pm_power_test_points
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.pm_power_tests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pm_report_id  uuid NOT NULL,
    instrument_id uuid,

    tested_by uuid,
    tested_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_pm_power_tests_report FOREIGN KEY (pm_report_id)
        REFERENCES rtu.pm_reports (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_pm_power_tests_instrument FOREIGN KEY (instrument_id)
        REFERENCES rtu.calibration_instruments (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uk_pm_power_tests_report_id UNIQUE (pm_report_id)
);

CREATE INDEX idx_pm_power_tests_instrument_id ON rtu.pm_power_tests (instrument_id);

CREATE TRIGGER trg_pm_power_tests_updated_at
    BEFORE UPDATE ON rtu.pm_power_tests
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

CREATE TABLE rtu.pm_power_test_points (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pm_power_test_id uuid NOT NULL,

    equipment_role varchar(20) NOT NULL,
    brand          varchar(255),
    model          varchar(255),
    input_accept_range  varchar(100),
    input_result_value  numeric(10, 3),
    input_unit          varchar(20) DEFAULT 'VAC',
    output_accept_range varchar(100),
    output_result_value numeric(10, 3),
    output_unit         varchar(20) DEFAULT 'VDC',
    result              varchar(20),
    corrective_action   text,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_power_test_points_test FOREIGN KEY (pm_power_test_id)
        REFERENCES rtu.pm_power_tests (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT uk_power_test_points_test_role UNIQUE (pm_power_test_id, equipment_role),
    CONSTRAINT ck_power_test_points_role CHECK (equipment_role IN ('CIRCUIT_BREAKER', 'DC_POWER_SUPPLY')),
    CONSTRAINT ck_power_test_points_result CHECK (result IS NULL OR result IN ('ACCEPT', 'NOT_ACCEPTED'))
);

CREATE INDEX idx_pm_power_test_points_test_id ON rtu.pm_power_test_points (pm_power_test_id);

CREATE TRIGGER trg_power_test_points_updated_at
    BEFORE UPDATE ON rtu.pm_power_test_points
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- cm_reports (3 origins: STANDALONE / PM_ONSITE_FIX / PM_ESCALATED —
-- distinguished at the application layer by (work_order_id, pm_report_id))
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.cm_reports (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_id       uuid,
    work_order_round_id uuid,
    pm_report_id        uuid,
    panel_id            uuid NOT NULL,
    panel_device_id     uuid,

    reported_by      uuid NOT NULL,
    tag_code         varchar(100),
    error_logs       text,
    problem_detail   text,
    root_cause       text,
    reference_info   text,
    corrective_action text,
    recommendation   text,
    pending_reason   text,

    repaired_by  uuid,
    reported_at  timestamptz,
    started_at   timestamptz,
    ended_at     timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_cm_reports_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_cm_reports_round FOREIGN KEY (work_order_round_id)
        REFERENCES rtu.work_order_rounds (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_cm_reports_pm_report FOREIGN KEY (pm_report_id)
        REFERENCES rtu.pm_reports (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_cm_reports_panel FOREIGN KEY (panel_id)
        REFERENCES rtu.panels (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_cm_reports_panel_device FOREIGN KEY (panel_device_id)
        REFERENCES rtu.panel_devices (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT uk_cm_reports_round_id UNIQUE (work_order_round_id),
    CONSTRAINT ck_cm_reports_origin CHECK (work_order_id IS NOT NULL OR pm_report_id IS NOT NULL)
);

CREATE INDEX idx_cm_reports_work_order_id ON rtu.cm_reports (work_order_id);
CREATE INDEX idx_cm_reports_pm_report_id ON rtu.cm_reports (pm_report_id);
CREATE INDEX idx_cm_reports_panel_id ON rtu.cm_reports (panel_id);
CREATE INDEX idx_cm_reports_panel_device_id ON rtu.cm_reports (panel_device_id);
CREATE INDEX idx_cm_reports_reported_by ON rtu.cm_reports (reported_by);

CREATE TRIGGER trg_cm_reports_updated_at
    BEFORE UPDATE ON rtu.cm_reports
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- attachments (polymorphic — replaces report_image/cm_image/photo_url etc.)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.attachments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type varchar(30) NOT NULL,
    entity_id   uuid NOT NULL,

    s3_bucket     varchar(100) NOT NULL,
    s3_key        varchar(500) NOT NULL,
    original_name varchar(255),
    mime_type     varchar(100) NOT NULL,
    file_size     bigint NOT NULL,
    caption       text,

    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid,

    CONSTRAINT ck_attachments_entity_type CHECK (entity_type IN
        ('WORK_ORDER', 'PM_REPORT', 'CM_REPORT', 'CALIBRATION', 'PM_GROUND_TEST', 'PM_POWER_TEST_POINT', 'PANEL_DEVICE')),
    CONSTRAINT ck_attachments_file_size CHECK (file_size > 0 AND file_size <= 10485760)
);

CREATE INDEX idx_attachments_entity ON rtu.attachments (entity_type, entity_id);
CREATE INDEX idx_attachments_created_at ON rtu.attachments (created_at DESC);

CREATE TRIGGER trg_attachments_updated_at
    BEFORE UPDATE ON rtu.attachments
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- notifications (System Design Screen 06)
-- ---------------------------------------------------------------------------
CREATE TABLE rtu.notifications (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    work_order_id uuid NOT NULL,
    recipient_id  uuid NOT NULL,

    type    varchar(20) NOT NULL,
    title   varchar(255),
    message text,

    is_read boolean NOT NULL DEFAULT false,
    read_at timestamptz,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid,
    updated_by uuid,

    CONSTRAINT fk_notifications_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT ck_notifications_type CHECK (type IN
        ('NEW_ASSIGNMENT', 'PENDING_WORK', 'PENDING_APPROVAL', 'COMPLETED', 'CM_PENDING'))
);

CREATE INDEX idx_notifications_work_order_id ON rtu.notifications (work_order_id);
CREATE INDEX idx_notifications_recipient_unread ON rtu.notifications (recipient_id, is_read);
CREATE INDEX idx_notifications_created_at ON rtu.notifications (created_at DESC);

CREATE TRIGGER trg_notifications_updated_at
    BEFORE UPDATE ON rtu.notifications
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

-- ---------------------------------------------------------------------------
-- extend rtu.calibrations for the PM 6-month calibration flow
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.calibrations
    ADD COLUMN work_order_id      uuid,
    ADD COLUMN pm_report_id       uuid,
    ADD COLUMN channel_type       varchar(20),
    ADD COLUMN eut_manufacturer   varchar(255),
    ADD COLUMN eut_model          varchar(255),
    ADD COLUMN eut_serial_no      varchar(255),
    ADD COLUMN eut_input_range    varchar(100),
    ADD COLUMN eut_accuracy_class varchar(100),
    ADD COLUMN eut_power_supply   varchar(100),
    ADD COLUMN eut_output_range   varchar(100),
    ADD COLUMN result_type        varchar(30),
    ADD COLUMN result_other_text  varchar(255);

ALTER TABLE rtu.calibrations
    ADD CONSTRAINT fk_calibrations_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    ADD CONSTRAINT fk_calibrations_pm_report FOREIGN KEY (pm_report_id)
        REFERENCES rtu.pm_reports (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    ADD CONSTRAINT ck_calibrations_channel_type
        CHECK (channel_type IS NULL OR channel_type IN ('PRESSURE', 'FLOW', 'LEVEL', 'RTU_READBACK')),
    ADD CONSTRAINT ck_calibrations_result_type
        CHECK (result_type IS NULL OR result_type IN ('TESTED', 'CALIBRATED_AND_TESTED', 'OTHER'));

CREATE INDEX idx_calibrations_work_order_id ON rtu.calibrations (work_order_id);
CREATE INDEX idx_calibrations_pm_report_id ON rtu.calibrations (pm_report_id);

-- ---------------------------------------------------------------------------
-- extend rtu.panels with PM scheduling denormalized dates
-- ---------------------------------------------------------------------------
ALTER TABLE rtu.panels
    ADD COLUMN install_date date,
    ADD COLUMN last_pm_date date,
    ADD COLUMN next_pm_date date;
