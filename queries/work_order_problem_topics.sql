-- name: InsertWorkOrderProblemTopic :exec
INSERT INTO rtu.work_order_problem_topics (work_order_id, problem_topic_id, sort_order)
VALUES (@work_order_id::uuid, @problem_topic_id::uuid, @sort_order::smallint);

-- name: DeleteWorkOrderProblemTopic :exec
DELETE FROM rtu.work_order_problem_topics
WHERE work_order_id = @work_order_id::uuid
  AND problem_topic_id = @problem_topic_id::uuid;

-- name: DeleteWorkOrderProblemTopicsByWorkOrder :exec
DELETE FROM rtu.work_order_problem_topics
WHERE work_order_id = @work_order_id::uuid;

-- name: WorkOrderHasProblemTopic :one
SELECT EXISTS (
    SELECT 1 FROM rtu.work_order_problem_topics
    WHERE work_order_id = @work_order_id::uuid
      AND problem_topic_id = @problem_topic_id::uuid
) AS found;

-- name: NextWorkOrderProblemTopicSortOrder :one
SELECT COALESCE(MAX(sort_order), -1)::smallint AS next_sort
FROM rtu.work_order_problem_topics
WHERE work_order_id = @work_order_id::uuid;

-- name: ListProblemTopicsByWorkOrder :many
SELECT pt.id, pt.code, pt.name, wopt.sort_order
FROM rtu.work_order_problem_topics wopt
JOIN rtu.problem_topics pt ON pt.id = wopt.problem_topic_id
WHERE wopt.work_order_id = @work_order_id::uuid
ORDER BY wopt.sort_order, pt.sort_order, pt.code;

-- name: ListProblemTopicsByWorkOrders :many
SELECT
    wopt.work_order_id,
    pt.id,
    pt.code,
    pt.name,
    wopt.sort_order
FROM rtu.work_order_problem_topics wopt
JOIN rtu.problem_topics pt ON pt.id = wopt.problem_topic_id
WHERE wopt.work_order_id = ANY(@work_order_ids::uuid[])
ORDER BY wopt.work_order_id, wopt.sort_order, pt.sort_order, pt.code;
