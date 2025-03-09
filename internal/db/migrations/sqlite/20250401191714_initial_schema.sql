-- migrate:up

-- Stores information about individuals/organizations reporting abuse cases
-- Includes contact info and optional user system linkage
create table if not exists abuse_reporters (
    id INTEGER primary key,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    email TEXT not null unique,
    name TEXT not null,
    user_id TEXT
);
create index if not exists idx_abuse_reporters_email
on abuse_reporters (email);

-- Catalog of subjects involved in abuse cases (hashed content or URLs)
-- Used for pattern detection and duplicate case prevention
create table if not exists abuse_subjects (
    id INTEGER primary key,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    identifier BLOB not null,
    type TEXT check (type in ('hash', 'url')) not null,
    source_url TEXT
);
create index if not exists idx_abuse_subjects_identifier_type
on abuse_subjects (identifier, type);

-- Main case tracking table with lifecycle management
-- References reporters, subjects, and assignees through foreign keys
-- Contains classification scores and risk assessment data
create table if not exists abuse_cases (
    id INTEGER primary key,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    reference_number TEXT not null unique,
    type TEXT check (
        type in ('spam', 'harassment', 'content', 'malware', 'other')
    ) not null,
    status TEXT check (
        status in ('new', 'in_progress', 'resolved', 'closed')
    ) not null,
    priority TEXT check (
        priority in ('low', 'medium', 'high', 'critical')
    ) not null,
    description TEXT not null,
    source TEXT check (source in ('web_form', 'email', 'api')) not null,
    is_duplicate BOOLEAN default FALSE,
    needs_review BOOLEAN default FALSE,
    content_hash TEXT not null,
    classification_scores TEXT,
    risk_factors TEXT,
    reporter_id INTEGER not null,
    subject_id INTEGER not null,
    assignee_id INTEGER,
    last_activity_at DATETIME not null,
    foreign key (reporter_id) references abuse_reporters (id),
    foreign key (subject_id) references abuse_subjects (id)
);
create index if not exists idx_abuse_cases_status
on abuse_cases (status);
create index if not exists idx_abuse_cases_priority
on abuse_cases (priority);
create index if not exists idx_abuse_cases_assignee_id
on abuse_cases (assignee_id);
create index if not exists idx_abuse_cases_last_activity_at
on abuse_cases (last_activity_at);
create index if not exists idx_abuse_cases_reporter_id
on abuse_cases (reporter_id);
create index if not exists idx_abuse_cases_subject_id
on abuse_cases (subject_id);
create index if not exists idx_abuse_cases_type
on abuse_cases (type);

-- Full audit trail of all case-related communications
-- Supports email threading and internal/external message tracking
create table if not exists abuse_communications (
    id INTEGER primary key,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    case_id INTEGER not null,
    sender_id INTEGER not null,
    type TEXT check (type in ('email', 'note', 'response')) not null,
    direction TEXT check (
        direction in ('incoming', 'outgoing', 'internal', 'external')
    ) not null,
    content TEXT not null,
    thread_id TEXT,
    parent_id INTEGER,
    foreign key (case_id) references abuse_cases (id),
    foreign key (parent_id) references abuse_communications (id)
);
create index if not exists idx_abuse_communications_case_id
on abuse_communications (case_id);
create index if not exists idx_abuse_communications_thread_id
on abuse_communications (thread_id);
create index if not exists idx_abuse_communications_parent_id
on abuse_communications (parent_id);
create index if not exists idx_abuse_communications_created_at
on abuse_communications (created_at);

-- Stores physical evidence files related to abuse cases
-- Tracks origin source and storage location metadata
create table if not exists abuse_evidence (
    id INTEGER primary key,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    case_id INTEGER not null,
    submitter_id INTEGER not null,
    file_name TEXT not null,
    content_type TEXT not null,
    file_size INTEGER not null,
    storage_path TEXT not null,
    source TEXT check (
        source in ('email', 'web_upload', 'api', 'system')
    ) not null,
    description TEXT,
    metadata TEXT,
    foreign key (case_id) references abuse_cases (id)
);
create index if not exists idx_abuse_evidence_case_id
on abuse_evidence (case_id);

-- Secure API token storage for case access authorization
-- Includes expiration and revocation tracking
create table if not exists abuse_tokens (
    id INTEGER primary key,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    case_id INTEGER not null,
    reporter_id INTEGER not null,
    token BLOB(32) unique not null,
    expires_at DATETIME,
    revoked_at DATETIME,
    last_used_at DATETIME,
    foreign key (case_id) references abuse_cases (id),
    foreign key (reporter_id) references abuse_reporters (id)
);
create index if not exists idx_abuse_tokens_case_id
on abuse_tokens (case_id);
create index if not exists idx_abuse_tokens_reporter_id
on abuse_tokens (reporter_id);
create index if not exists idx_abuse_tokens_expires_at
on abuse_tokens (expires_at);

-- Central registry of blocked content with enforcement policies
-- Tracks content hash, moderation decisions, and case associations
CREATE TABLE IF NOT EXISTS abuse_blocklist (
                                                     id INTEGER PRIMARY KEY,
                                                     created_at DATETIME,
                                                     updated_at DATETIME,
                                                     deleted_at DATETIME,
                                                     hash BLOB NOT NULL,
                                                     mime_type TEXT,
                                                     size INTEGER,
                                                     file_name TEXT,
                                                     uploader_id INTEGER,
                                                     reason TEXT CHECK (reason IN ('malware', 'csam', 'copyright', 'harassment', 'hate_speech', 'spam', 'policy', 'manual')) NOT NULL,
    severity TEXT CHECK (severity IN ('critical', 'high', 'medium', 'low')) NOT NULL,
    action TEXT CHECK (action IN ('reject', 'quarantine', 'warn', 'log')) NOT NULL,
    description TEXT,
    blocked_by INTEGER NOT NULL,
    source TEXT CHECK (source IN ('scanner', 'report', 'admin', 'external')) NOT NULL,
    case_id INTEGER,
    expires_at DATETIME,
    reviewed_at DATETIME,
    metadata TEXT,
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id)
    );
CREATE INDEX IF NOT EXISTS idx_abuse_blocklist_hash
    ON abuse_blocklist (hash);
CREATE INDEX IF NOT EXISTS idx_abuse_blocklist_case_id
    ON abuse_blocklist (case_id);
CREATE INDEX IF NOT EXISTS idx_abuse_blocklist_expires_at
    ON abuse_blocklist (expires_at);
CREATE INDEX IF NOT EXISTS idx_abuse_blocklist_reason
    ON abuse_blocklist (reason);

-- Scheduled scanning operations for case content verification
-- Maintains scan history and results for auditing purposes
CREATE TABLE IF NOT EXISTS abuse_case_scans (
                                                id INTEGER PRIMARY KEY,
                                                created_at DATETIME,
                                                updated_at DATETIME,
                                                deleted_at DATETIME,
                                                case_id INTEGER NOT NULL,
                                                subject_id INTEGER NOT NULL,
                                                status TEXT CHECK (status IN ('pending', 'scanning', 'clean', 'flagged', 'error')) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    requested_by INTEGER NOT NULL,
    scheduled_for DATETIME NOT NULL,
    last_attempt DATETIME,
    scan_results TEXT,
    FOREIGN KEY (case_id) REFERENCES abuse_cases(id),
    FOREIGN KEY (subject_id) REFERENCES abuse_subjects(id)
    );
CREATE INDEX IF NOT EXISTS idx_abuse_case_scans_case_id
    ON abuse_case_scans (case_id);

-- Case status history tracking
CREATE TABLE IF NOT EXISTS case_status_histories (
    id INTEGER PRIMARY KEY,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    case_id INTEGER NOT NULL,
    old_status TEXT CHECK (old_status IN ('new', 'in_progress', 'resolved', 'closed')),
    new_status TEXT CHECK (new_status IN ('new', 'in_progress', 'resolved', 'closed')),
    changed_at DATETIME NOT NULL,
    changed_by INTEGER NOT NULL,
    FOREIGN KEY (case_id) REFERENCES abuse_cases(id)
);
CREATE INDEX IF NOT EXISTS idx_case_status_histories_case_id 
    ON case_status_histories (case_id);
CREATE INDEX IF NOT EXISTS idx_case_status_histories_changed_at 
    ON case_status_histories (changed_at);

-- Daily case resolution metrics
CREATE VIEW IF NOT EXISTS abuse_daily_resolutions AS
SELECT 
    DATE(updated_at) AS resolution_date,
    COUNT(*) AS resolved_count,
    AVG(JULIANDAY(updated_at) - JULIANDAY(created_at)) AS avg_resolution_days
FROM abuse_cases
WHERE status IN ('resolved', 'closed')
GROUP BY resolution_date;
CREATE INDEX IF NOT EXISTS idx_abuse_case_scans_subject_id
    ON abuse_case_scans (subject_id);
CREATE INDEX IF NOT EXISTS idx_abuse_case_scans_status_priority
    ON abuse_case_scans (status, priority);
CREATE INDEX IF NOT EXISTS idx_abuse_case_scans_scheduled_for
    ON abuse_case_scans (scheduled_for);


-- migrate:down

DROP VIEW IF EXISTS abuse_daily_resolutions;
DROP TABLE IF EXISTS case_status_histories;
DROP INDEX IF EXISTS idx_case_status_histories_case_id;
DROP INDEX IF EXISTS idx_case_status_histories_changed_at;
DROP INDEX IF EXISTS idx_abuse_case_scans_scheduled_for;
DROP INDEX IF EXISTS idx_abuse_case_scans_status_priority;
DROP INDEX IF EXISTS idx_abuse_case_scans_subject_id;
DROP INDEX IF EXISTS idx_abuse_case_scans_case_id;
DROP TABLE IF EXISTS abuse_case_scans;

DROP INDEX IF EXISTS idx_abuse_blocklist_reason;
DROP INDEX IF EXISTS idx_abuse_blocklist_expires_at;
DROP INDEX IF EXISTS idx_abuse_blocklist_case_id;
DROP INDEX IF EXISTS idx_abuse_blocklist_hash;
DROP TABLE IF EXISTS abuse_blocklist;

DROP INDEX IF EXISTS idx_abuse_tokens_expires_at;
DROP INDEX IF EXISTS idx_abuse_tokens_reporter_id;
DROP INDEX IF EXISTS idx_abuse_tokens_case_id;
DROP TABLE IF EXISTS abuse_tokens;

DROP INDEX IF EXISTS idx_abuse_evidence_case_id;
DROP TABLE IF EXISTS abuse_evidence;

DROP INDEX IF EXISTS idx_abuse_communications_created_at;
DROP INDEX IF EXISTS idx_abuse_communications_parent_id;
DROP INDEX IF EXISTS idx_abuse_communications_thread_id;
DROP INDEX IF EXISTS idx_abuse_communications_case_id;
DROP TABLE IF EXISTS abuse_communications;

DROP INDEX IF EXISTS idx_abuse_cases_type;
DROP INDEX IF EXISTS idx_abuse_cases_subject_id;
DROP INDEX IF EXISTS idx_abuse_cases_reporter_id;
DROP INDEX IF EXISTS idx_abuse_cases_last_activity_at;
DROP INDEX IF EXISTS idx_abuse_cases_assignee_id;
DROP INDEX IF EXISTS idx_abuse_cases_priority;
DROP INDEX IF EXISTS idx_abuse_cases_status;
DROP TABLE IF EXISTS abuse_cases;

DROP INDEX IF EXISTS idx_abuse_subjects_identifier_type;
DROP TABLE IF EXISTS abuse_subjects;

DROP INDEX IF EXISTS idx_abuse_reporters_email;
DROP TABLE IF EXISTS abuse_reporters;
