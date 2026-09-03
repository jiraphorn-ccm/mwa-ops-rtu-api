-- Denormalize CM problem topic onto work_orders for duplicate checks and list views.
-- Topic remains on cm_reports per round; work_orders.problem_topic_id is the canonical
-- topic for the CM work order (set at create, synced when the current round report changes).

ALTER TABLE rtu.work_orders
    ADD COLUMN problem_topic_id uuid,
    ADD CONSTRAINT fk_work_orders_problem_topic
        FOREIGN KEY (problem_topic_id)
        REFERENCES rtu.problem_topics (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    ADD CONSTRAINT ck_work_orders_pm_no_problem_topic CHECK (
        work_order_type <> 'PM' OR problem_topic_id IS NULL
    );

-- Backfill from the latest cm_report row per CM work order.
UPDATE rtu.work_orders wo
SET problem_topic_id = sub.problem_topic_id
FROM (
    SELECT DISTINCT ON (cr.work_order_id)
        cr.work_order_id,
        cr.problem_topic_id
    FROM rtu.cm_reports cr
    WHERE cr.work_order_id IS NOT NULL
      AND cr.problem_topic_id IS NOT NULL
    ORDER BY cr.work_order_id, cr.created_at DESC
) sub
WHERE wo.id = sub.work_order_id
  AND wo.work_order_type = 'CM';

CREATE INDEX idx_work_orders_cm_panel_topic
    ON rtu.work_orders (panel_id, problem_topic_id)
    WHERE work_order_type = 'CM'
      AND active = true
      AND status IN ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL');
