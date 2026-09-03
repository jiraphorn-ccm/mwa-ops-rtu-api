-- CM work orders may cover multiple problem topics (junction table).
-- Duplicate checks use this table — one open CM per panel per topic.

CREATE TABLE IF NOT EXISTS rtu.work_order_problem_topics (
    work_order_id    uuid        NOT NULL,
    problem_topic_id uuid        NOT NULL,
    sort_order       smallint    NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_work_order_problem_topics PRIMARY KEY (work_order_id, problem_topic_id),
    CONSTRAINT fk_wopt_work_order FOREIGN KEY (work_order_id)
        REFERENCES rtu.work_orders (id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_wopt_problem_topic FOREIGN KEY (problem_topic_id)
        REFERENCES rtu.problem_topics (id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_wopt_problem_topic ON rtu.work_order_problem_topics (problem_topic_id);
CREATE INDEX IF NOT EXISTS idx_wopt_work_order ON rtu.work_order_problem_topics (work_order_id);

-- Backfill from work_orders.problem_topic_id when migration 000009 was applied.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'rtu'
          AND table_name = 'work_orders'
          AND column_name = 'problem_topic_id'
    ) THEN
        INSERT INTO rtu.work_order_problem_topics (work_order_id, problem_topic_id, sort_order)
        SELECT wo.id, wo.problem_topic_id, 0
        FROM rtu.work_orders wo
        WHERE wo.problem_topic_id IS NOT NULL
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

INSERT INTO rtu.work_order_problem_topics (work_order_id, problem_topic_id, sort_order)
SELECT DISTINCT cr.work_order_id, cr.problem_topic_id, 0
FROM rtu.cm_reports cr
WHERE cr.work_order_id IS NOT NULL
  AND cr.problem_topic_id IS NOT NULL
ON CONFLICT DO NOTHING;

DROP INDEX IF EXISTS rtu.idx_work_orders_cm_panel_topic;

ALTER TABLE rtu.work_orders
    DROP CONSTRAINT IF EXISTS fk_work_orders_problem_topic,
    DROP CONSTRAINT IF EXISTS ck_work_orders_pm_no_problem_topic;

ALTER TABLE rtu.work_orders
    DROP COLUMN IF EXISTS problem_topic_id;
