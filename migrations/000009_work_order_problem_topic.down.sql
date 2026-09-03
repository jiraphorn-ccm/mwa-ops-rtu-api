DROP INDEX IF EXISTS rtu.idx_work_orders_cm_panel_topic;

ALTER TABLE rtu.work_orders
    DROP CONSTRAINT IF EXISTS ck_work_orders_pm_no_problem_topic,
    DROP CONSTRAINT IF EXISTS fk_work_orders_problem_topic,
    DROP COLUMN IF EXISTS problem_topic_id;
