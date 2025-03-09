-- migrate:up

-- Stores reporter information with system user linkage
-- Uses InnoDB for ACID compliance on critical reporting data
CREATE TABLE IF NOT EXISTS abuse_reporters (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    user_id VARCHAR(255)
) ENGINE = InnoDB;

-- Central registry of abuse targets (hashed content/URLs)
-- ENUM ensures valid type classification for analysis
CREATE TABLE IF NOT EXISTS abuse_subjects (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    identifier BLOB NOT NULL,
    `type` ENUM('hash', 'url') NOT NULL,
    source_url VARCHAR(511)
) ENGINE = InnoDB;

-- Core case management table with JSON-encoded risk analysis
-- Maintains strict referential integrity through InnoDB FKs
CREATE TABLE IF NOT EXISTS abuse_cases (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    reference_number VARCHAR(255) NOT NULL UNIQUE,
    `type` ENUM('spam', 'harassment', 'content', 'malware', 'other') NOT NULL,
    status ENUM('new', 'in_progress', 'resolved', 'closed') NOT NULL,
    priority ENUM('low', 'medium', 'high', 'critical') NOT NULL,
    description TEXT NOT NULL,
    source ENUM('web_form', 'email', 'api') NOT NULL,
    is_duplicate BOOLEAN DEFAULT FALSE,
    needs_review BOOLEAN DEFAULT FALSE,
    content_hash VARCHAR(255) NOT NULL,
    classification_scores JSON,
    risk_factors JSON,
    reporter_id INT NOT NULL,
    subject_id INT NOT NULL,
    assignee_id INT,
    last_activity_at DATETIME (6) NOT NULL,
    FOREIGN KEY (reporter_id) REFERENCES abuse_reporters (id),
    FOREIGN KEY (subject_id) REFERENCES abuse_subjects (id)
) ENGINE = InnoDB;

-- Full communication history with thread management
-- ENUM types enforce consistent direction/type classification
CREATE TABLE IF NOT EXISTS abuse_communications (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    case_id INT NOT NULL,
    sender_id INT NOT NULL,
    `type` ENUM('email', 'note', 'response') NOT NULL,
    direction ENUM('incoming', 'outgoing', 'internal', 'external') NOT NULL,
    content TEXT NOT NULL,
    thread_id VARCHAR(255),
    parent_id INT,
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id)
) ENGINE = InnoDB;

-- Evidence repository with storage system metadata
-- Uses VARCHAR size limits appropriate for path structures
CREATE TABLE IF NOT EXISTS abuse_evidence (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    case_id INT NOT NULL,
    submitter_id INT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(127) NOT NULL,
    file_size BIGINT NOT NULL,
    storage_path VARCHAR(511) NOT NULL,
    source ENUM('email', 'web_upload', 'api', 'system') NOT NULL,
    description TEXT,
    metadata JSON,
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id)
) ENGINE = InnoDB;

-- Secure token storage with binary(32) for cryptographic safety
-- Automatic index optimization via InnoDB clustering
CREATE TABLE IF NOT EXISTS abuse_tokens (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    case_id INT NOT NULL,
    reporter_id INT NOT NULL,
    token BINARY(32) UNIQUE NOT NULL,
    expires_at DATETIME (6),
    revoked_at DATETIME (6),
    last_used_at DATETIME (6),
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id),
    FOREIGN KEY (reporter_id) REFERENCES abuse_reporters (id)
) ENGINE = InnoDB;

-- Content moderation registry with enforcement policies
-- JSON metadata field allows flexible rule storage
CREATE TABLE IF NOT EXISTS abuse_blocked_content (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    hash BLOB NOT NULL,
    mime_type VARCHAR(127),
    size BIGINT,
    file_name VARCHAR(255),
    uploader_id INT,
    reason ENUM(
        'malware',
        'csam',
        'copyright',
        'harassment',
        'hate_speech',
        'spam',
        'policy',
        'manual'
    ) NOT NULL,
    severity ENUM('critical', 'high', 'medium', 'low') NOT NULL,
    action ENUM('reject', 'quarantine', 'warn', 'log') NOT NULL,
    description TEXT,
    blocked_by INT NOT NULL,
    source ENUM('scanner', 'report', 'admin', 'external') NOT NULL,
    case_id INT, -- Nullable FK
    expires_at DATETIME (6),
    reviewed_at DATETIME (6),
    metadata JSON,
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id)
) ENGINE = InnoDB;

-- Automated scanning schedule with result tracking
-- Uses JSON field for detailed scan result storage
CREATE TABLE IF NOT EXISTS abuse_case_scans (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    case_id INT NOT NULL,
    subject_id INT NOT NULL,
    status ENUM('pending', 'scanning', 'clean', 'flagged', 'error') NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    requested_by INT NOT NULL,
    scheduled_for DATETIME (6) NOT NULL,
    last_attempt DATETIME (6),
    scan_results JSON,
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id),
    FOREIGN KEY (subject_id) REFERENCES abuse_subjects (id)
) ENGINE = InnoDB;

-- Case status history tracking
CREATE TABLE IF NOT EXISTS case_status_histories (
    id INT PRIMARY KEY AUTO_INCREMENT,
    created_at DATETIME (6),
    updated_at DATETIME (6),
    deleted_at DATETIME (6),
    case_id INT NOT NULL,
    old_status ENUM('new', 'in_progress', 'resolved', 'closed'),
    new_status ENUM('new', 'in_progress', 'resolved', 'closed'),
    changed_at DATETIME (6) NOT NULL,
    changed_by INT NOT NULL,
    FOREIGN KEY (case_id) REFERENCES abuse_cases (id)
) ENGINE = InnoDB;
CREATE INDEX idx_case_status_histories_case_id 
    ON case_status_histories (case_id);
CREATE INDEX idx_case_status_histories_changed_at 
    ON case_status_histories (changed_at);

-- Daily resolution metrics view
CREATE VIEW IF NOT EXISTS abuse_daily_resolutions AS
SELECT 
    DATE(updated_at) AS resolution_date,
    COUNT(*) AS resolved_count,
    AVG(TIMESTAMPDIFF(SECOND, created_at, updated_at)) AS avg_resolution_seconds
FROM abuse_cases
WHERE status IN ('resolved', 'closed')
GROUP BY resolution_date;

-- migrate:down

-- Drop tables in reverse order of creation to respect FK constraints
DROP VIEW IF EXISTS abuse_daily_resolutions;
DROP TABLE IF EXISTS case_status_histories;
DROP TABLE IF EXISTS abuse_case_scans;
DROP TABLE IF EXISTS abuse_blocked_content;
DROP TABLE IF EXISTS abuse_tokens;
DROP TABLE IF EXISTS abuse_evidence;
DROP TABLE IF EXISTS abuse_communications;
DROP TABLE IF EXISTS abuse_cases;
DROP TABLE IF EXISTS abuse_subjects;
DROP TABLE IF EXISTS abuse_reporters;
