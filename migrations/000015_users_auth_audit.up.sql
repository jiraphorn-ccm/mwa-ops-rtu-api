-- Users, refresh sessions and HTTP audit trail.
-- No RBAC: this service only needs to know who acted.

CREATE TABLE rtu.users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_code   varchar(20)  NOT NULL,
    title           varchar(10),
    first_name      varchar(100) NOT NULL,
    last_name       varchar(100) NOT NULL,
    email           varchar(100) NOT NULL,
    password_hash   text         NOT NULL,
    position        varchar(150),
    active          boolean      NOT NULL DEFAULT true,
    last_login_at   timestamptz,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),
    created_by      uuid,
    updated_by      uuid,
    CONSTRAINT uk_users_employee_code UNIQUE (employee_code),
    CONSTRAINT uk_users_email UNIQUE (email),
    CONSTRAINT fk_users_created_by FOREIGN KEY (created_by)
        REFERENCES rtu.users (id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT fk_users_updated_by FOREIGN KEY (updated_by)
        REFERENCES rtu.users (id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE UNIQUE INDEX uk_users_email_lower ON rtu.users (lower(email));
CREATE INDEX idx_users_active ON rtu.users (active);
CREATE INDEX idx_users_created_at ON rtu.users (created_at DESC);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON rtu.users
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

CREATE TABLE rtu.refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid         NOT NULL,
    token_hash  varchar(64)  NOT NULL,
    ip_address  varchar(45),
    user_agent  text,
    revoked_at  timestamptz,
    expires_at  timestamptz  NOT NULL,
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT uk_refresh_tokens_hash UNIQUE (token_hash),
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id)
        REFERENCES rtu.users (id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX idx_refresh_tokens_user ON rtu.refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_expires ON rtu.refresh_tokens (expires_at);
CREATE INDEX idx_refresh_tokens_active ON rtu.refresh_tokens (user_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE TRIGGER trg_refresh_tokens_updated_at
    BEFORE UPDATE ON rtu.refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION rtu.set_updated_at();

CREATE TABLE rtu.audit_logs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid,
    action       varchar(50) NOT NULL,
    method       varchar(10) NOT NULL,
    path         text        NOT NULL,
    resource     varchar(100),
    resource_id  uuid,
    status_code  integer,
    ip_address   varchar(45),
    user_agent   text,
    request_id   varchar(64),
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_audit_logs_user FOREIGN KEY (user_id)
        REFERENCES rtu.users (id) ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE INDEX idx_audit_logs_user ON rtu.audit_logs (user_id);
CREATE INDEX idx_audit_logs_action ON rtu.audit_logs (action);
CREATE INDEX idx_audit_logs_resource ON rtu.audit_logs (resource, resource_id);
CREATE INDEX idx_audit_logs_created ON rtu.audit_logs (created_at DESC);
