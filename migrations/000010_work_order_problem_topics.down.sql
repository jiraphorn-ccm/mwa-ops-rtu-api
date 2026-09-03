ALTER TABLE rtu.work_orders
    ADD COLUMN problem_topic_id uuid,
    ADD CONSTRAINT fk_work_orders_problem_topic
        FOREIGN KEY (problem_topic_id)
        REFERENCES rtu.problem_topics (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    ADD CONSTRAINT ck_work_orders_pm_no_problem_topic CHECK (
        work_order_type <> 'PM' OR problem_topic_id IS NULL
    );

UPDATE rtu.work_orders wo
SET problem_topic_id = sub.problem_topic_id
FROM (
    SELECT DISTINCT ON (wopt.work_order_id)
        wopt.work_order_id,
        wopt.problem_topic_id
    FROM rtu.work_order_problem_topics wopt
    ORDER BY wopt.work_order_id, wopt.sort_order, wopt.created_at
) sub
WHERE wo.id = sub.work_order_id;

CREATE INDEX idx_work_orders_cm_panel_topic
    ON rtu.work_orders (panel_id, problem_topic_id)
    WHERE work_order_type = 'CM'
      AND active = true
      AND status IN ('ASSIGNED', 'IN_PROGRESS', 'PENDING', 'PENDING_APPROVAL');

DROP TABLE IF EXISTS rtu.work_order_problem_topics;
