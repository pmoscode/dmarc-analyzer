-- Initial schema — IMPLEMENTIERUNG.md section 8.1.
-- Timestamps for report periods are Unix seconds in UTC.

CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL,
    host          TEXT NOT NULL,
    port          INTEGER NOT NULL,
    username      TEXT NOT NULL,
    mailbox       TEXT NOT NULL DEFAULT 'INBOX',
    use_tls       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL
    -- Password deliberately NOT stored here, but in the keychain (AP 3).
);

CREATE TABLE sync_state (
    account_id    TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    mailbox       TEXT NOT NULL,
    uid_validity  INTEGER NOT NULL,
    last_uid      INTEGER NOT NULL,
    last_sync_at  TEXT,
    PRIMARY KEY (account_id, mailbox)
);

CREATE TABLE reports (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    org_name      TEXT NOT NULL,
    org_email     TEXT,
    org_extra_contact_info TEXT,
    report_id     TEXT NOT NULL,
    date_begin    INTEGER NOT NULL,
    date_end      INTEGER NOT NULL,
    policy_domain TEXT NOT NULL,
    policy_p      TEXT, policy_sp TEXT,
    policy_adkim  TEXT, policy_aspf TEXT,
    policy_pct    INTEGER, policy_fo TEXT,
    account_id    TEXT REFERENCES accounts(id) ON DELETE SET NULL,
    mailbox       TEXT,
    message_uid   INTEGER,
    filename      TEXT,
    imported_at   TEXT NOT NULL,
    UNIQUE (org_name, report_id, date_begin)
);

CREATE TABLE report_errors (
    report_id     INTEGER NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    message       TEXT NOT NULL
);

CREATE TABLE records (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id     INTEGER NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    source_ip     TEXT NOT NULL,
    message_count INTEGER NOT NULL,
    disposition   TEXT NOT NULL,
    dkim_result   TEXT NOT NULL,      -- evaluated (aligned)
    spf_result    TEXT NOT NULL,
    header_from   TEXT NOT NULL,
    envelope_from TEXT,
    envelope_to   TEXT
);

CREATE TABLE record_reasons (
    record_id     INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    type          TEXT,
    comment       TEXT
);

CREATE TABLE auth_results_dkim (
    record_id     INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    domain        TEXT,
    selector      TEXT,
    result        TEXT,
    human_result  TEXT
);

CREATE TABLE auth_results_spf (
    record_id     INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    domain        TEXT,
    scope         TEXT,
    result        TEXT
);

CREATE TABLE raw_reports (
    report_id     INTEGER PRIMARY KEY REFERENCES reports(id) ON DELETE CASCADE,
    filename      TEXT,
    content       BLOB
);

CREATE TABLE failed_imports (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id    TEXT,
    message_uid   INTEGER,
    filename      TEXT,
    error         TEXT,
    raw           BLOB,
    occurred_at   TEXT NOT NULL
);

CREATE INDEX idx_reports_period   ON reports(date_begin, date_end);
CREATE INDEX idx_reports_domain   ON reports(policy_domain);
CREATE INDEX idx_reports_org      ON reports(org_name);
CREATE INDEX idx_records_report   ON records(report_id);
CREATE INDEX idx_records_ip       ON records(source_ip);
CREATE INDEX idx_records_from     ON records(header_from);

-- Without these four indexes, the batch-loading functions (reportrecords.go,
-- IN clause over record_id/report_id) effectively scan the whole table for
-- reports with many records — measured with 10,000 records: FindByID > 3s
-- instead of < 100ms.
CREATE INDEX idx_record_reasons_record     ON record_reasons(record_id);
CREATE INDEX idx_auth_results_dkim_record  ON auth_results_dkim(record_id);
CREATE INDEX idx_auth_results_spf_record   ON auth_results_spf(record_id);
CREATE INDEX idx_report_errors_report      ON report_errors(report_id);
