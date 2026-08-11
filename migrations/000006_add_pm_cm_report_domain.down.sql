-- Reverse of 000006_add_pm_cm_report_domain.up.sql.

ALTER TABLE rtu.panels
    DROP COLUMN IF EXISTS next_pm_date,
    DROP COLUMN IF EXISTS last_pm_date,
    DROP COLUMN IF EXISTS install_date;

DROP INDEX IF EXISTS rtu.idx_calibrations_pm_report_id;
DROP INDEX IF EXISTS rtu.idx_calibrations_work_order_id;

ALTER TABLE rtu.calibrations
    DROP CONSTRAINT IF EXISTS ck_calibrations_result_type,
    DROP CONSTRAINT IF EXISTS ck_calibrations_channel_type,
    DROP CONSTRAINT IF EXISTS fk_calibrations_pm_report,
    DROP CONSTRAINT IF EXISTS fk_calibrations_work_order;

ALTER TABLE rtu.calibrations
    DROP COLUMN IF EXISTS result_other_text,
    DROP COLUMN IF EXISTS result_type,
    DROP COLUMN IF EXISTS eut_output_range,
    DROP COLUMN IF EXISTS eut_power_supply,
    DROP COLUMN IF EXISTS eut_accuracy_class,
    DROP COLUMN IF EXISTS eut_input_range,
    DROP COLUMN IF EXISTS eut_serial_no,
    DROP COLUMN IF EXISTS eut_model,
    DROP COLUMN IF EXISTS eut_manufacturer,
    DROP COLUMN IF EXISTS channel_type,
    DROP COLUMN IF EXISTS pm_report_id,
    DROP COLUMN IF EXISTS work_order_id;

DROP TABLE IF EXISTS rtu.notifications;
DROP TABLE IF EXISTS rtu.attachments;
DROP TABLE IF EXISTS rtu.cm_reports;
DROP TABLE IF EXISTS rtu.pm_power_test_points;
DROP TABLE IF EXISTS rtu.pm_power_tests;
DROP TABLE IF EXISTS rtu.pm_ground_tests;
DROP TABLE IF EXISTS rtu.checklist_results;
DROP TABLE IF EXISTS rtu.pm_reports;
DROP TABLE IF EXISTS rtu.wo_approvals;
DROP TABLE IF EXISTS rtu.work_order_activity_logs;

ALTER TABLE rtu.work_orders DROP CONSTRAINT IF EXISTS fk_work_orders_current_round;

DROP TABLE IF EXISTS rtu.work_order_rounds;
DROP TABLE IF EXISTS rtu.work_orders;
DROP TABLE IF EXISTS rtu.checklist_items;
DROP TABLE IF EXISTS rtu.engineers;
