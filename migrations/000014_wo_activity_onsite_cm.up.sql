-- Allow ONSITE_CM_OPENED on work_order_activity_logs (PM onsite repair opened as CM WO).
ALTER TABLE rtu.work_order_activity_logs
    DROP CONSTRAINT IF EXISTS ck_wo_activity_logs_action;

ALTER TABLE rtu.work_order_activity_logs
    ADD CONSTRAINT ck_wo_activity_logs_action CHECK (action IN
        ('ASSIGNED', 'REASSIGNED', 'CHECKED_IN', 'CHECKED_OUT', 'SUBMITTED',
         'STATUS_CHANGED', 'APPROVED', 'APPROVED_COND', 'REJECTED', 'CANCELLED',
         'CM_SPAWNED', 'ONSITE_CM_OPENED'));
