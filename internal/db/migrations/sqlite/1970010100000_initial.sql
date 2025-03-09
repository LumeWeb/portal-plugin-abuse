-- Initial database schema for the template plugin
-- This migration establishes the base schema for storing items
--
-- Usage:
-- This migration runs automatically when the plugin is first installed
-- or when the database is initialized. It is idempotent and can be
-- run multiple times safely.
--
-- Tables:
-- items: Stores the basic item information with timestamps for tracking
--        creation, updates, and soft deletes
-- SQLite version of the schema

CREATE TABLE IF NOT EXISTS items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,          -- Unique identifier for each item
    name TEXT NOT NULL,                           -- Required item name
    description TEXT,                             -- Optional item description
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP, -- Creation timestamp
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, -- Last update timestamp
    deleted_at DATETIME NULL                      -- Soft delete support
);
