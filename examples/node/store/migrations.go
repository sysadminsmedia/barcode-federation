package store

import "fmt"

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS actors (
		id TEXT PRIMARY KEY,
		profile TEXT,
		display_name TEXT,
		node TEXT NOT NULL,
		public_key_pem TEXT NOT NULL,
		capabilities TEXT NOT NULL DEFAULT '[]',
		namespace_claims TEXT DEFAULT '[]',
		api_token TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		private_key_pem TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS records (
		id TEXT PRIMARY KEY,
		cbi TEXT NOT NULL,
		symbology TEXT NOT NULL,
		value TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		submitted_by TEXT NOT NULL,
		submitted_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		expires_at TEXT,
		origin_node TEXT NOT NULL,
		namespace_claim TEXT,
		metadata_schema TEXT NOT NULL,
		metadata_schema_digest TEXT,
		metadata TEXT NOT NULL,
		gs1_cross_ref TEXT,
		lifecycle_schema TEXT,
		signature TEXT NOT NULL,
		raw_json TEXT NOT NULL,
		verification_level INTEGER DEFAULT -1,
		FOREIGN KEY (submitted_by) REFERENCES actors(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_records_cbi ON records(cbi)`,
	`CREATE INDEX IF NOT EXISTS idx_records_status ON records(status)`,
	`CREATE INDEX IF NOT EXISTS idx_records_submitted_by ON records(submitted_by)`,
	`CREATE INDEX IF NOT EXISTS idx_records_value ON records(value)`,
	`CREATE INDEX IF NOT EXISTS idx_records_symbology ON records(symbology)`,

	`CREATE TABLE IF NOT EXISTS revision_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		record_id TEXT NOT NULL,
		revision_id TEXT NOT NULL,
		at TEXT NOT NULL,
		by_actor TEXT NOT NULL,
		change_type TEXT NOT NULL,
		summary TEXT,
		previous_revision_signature TEXT,
		FOREIGN KEY (record_id) REFERENCES records(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_revision_record ON revision_history(record_id)`,

	`CREATE TABLE IF NOT EXISTS namespace_claims (
		id TEXT PRIMARY KEY,
		claimed_by TEXT NOT NULL,
		namespace_type TEXT NOT NULL,
		scope TEXT NOT NULL,
		issued_at TEXT NOT NULL,
		expires_at TEXT,
		evidence TEXT NOT NULL,
		signature TEXT NOT NULL,
		raw_json TEXT NOT NULL,
		FOREIGN KEY (claimed_by) REFERENCES actors(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_claims_claimed_by ON namespace_claims(claimed_by)`,

	`CREATE TABLE IF NOT EXISTS federation_outbox (
		id TEXT PRIMARY KEY,
		activity_type TEXT NOT NULL,
		message_json TEXT NOT NULL,
		published_at TEXT NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_outbox_published ON federation_outbox(published_at)`,

	`CREATE TABLE IF NOT EXISTS federation_inbox (
		id TEXT PRIMARY KEY,
		sender TEXT NOT NULL,
		activity_type TEXT NOT NULL,
		message_json TEXT NOT NULL,
		received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		processed INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS delivery_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_node TEXT NOT NULL,
		message_id TEXT NOT NULL,
		message_json TEXT NOT NULL,
		next_attempt_at TEXT NOT NULL,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		last_error TEXT,
		FOREIGN KEY (message_id) REFERENCES federation_outbox(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_delivery_next ON delivery_queue(next_attempt_at)`,

	`CREATE TABLE IF NOT EXISTS peers (
		node_id TEXT PRIMARY KEY,
		meta_url TEXT NOT NULL,
		relationship TEXT NOT NULL DEFAULT 'known',
		added_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		last_seen TEXT,
		federation_policy TEXT DEFAULT 'open',
		public_key_pem TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS subscriptions (
		id TEXT PRIMARY KEY,
		actor_id TEXT NOT NULL,
		scope TEXT NOT NULL,
		callback_url TEXT NOT NULL,
		events TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		FOREIGN KEY (actor_id) REFERENCES actors(id)
	)`,

	`CREATE TABLE IF NOT EXISTS bulk_jobs (
		id TEXT PRIMARY KEY,
		actor_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		total INTEGER NOT NULL DEFAULT 0,
		processed INTEGER NOT NULL DEFAULT 0,
		failed INTEGER NOT NULL DEFAULT 0,
		errors TEXT DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		completed_at TEXT,
		FOREIGN KEY (actor_id) REFERENCES actors(id)
	)`,

	`CREATE TABLE IF NOT EXISTS successions (
		id TEXT PRIMARY KEY,
		old_fni TEXT NOT NULL,
		successor_fni TEXT NOT NULL,
		declared_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		document_json TEXT NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_successions_old_fni ON successions(old_fni)`,

	`CREATE TABLE IF NOT EXISTS schema_cache (
		url TEXT PRIMARY KEY,
		digest TEXT,
		content TEXT NOT NULL,
		fetched_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	)`,

	`CREATE TABLE IF NOT EXISTS pending_verifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		retract_message_json TEXT NOT NULL,
		succession_url TEXT NOT NULL,
		next_retry_at TEXT NOT NULL,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	)`,

	`CREATE TABLE IF NOT EXISTS verification_sessions (
		id TEXT PRIMARY KEY,
		actor_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		claimed_prefixes TEXT NOT NULL DEFAULT '[]',
		evidence_package TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		result TEXT,
		FOREIGN KEY (actor_id) REFERENCES actors(id)
	)`,

	// Custody transfer tracking
	`CREATE TABLE IF NOT EXISTS custody_transfers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		record_id TEXT NOT NULL,
		from_actor TEXT NOT NULL,
		to_actor TEXT NOT NULL,
		transferred_at TEXT NOT NULL,
		condition TEXT NOT NULL DEFAULT 'accepted',
		delegate_expiry TEXT,
		from_signature TEXT NOT NULL,
		to_signature TEXT,
		accepted_at TEXT,
		message_id TEXT,
		FOREIGN KEY (record_id) REFERENCES records(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_custody_record ON custody_transfers(record_id)`,
	`CREATE INDEX IF NOT EXISTS idx_custody_to_actor ON custody_transfers(to_actor)`,

	// Related records index for cross-CBI queries
	`CREATE TABLE IF NOT EXISTS related_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		record_id TEXT NOT NULL,
		related_cbi TEXT NOT NULL,
		relationship TEXT NOT NULL,
		quantity INTEGER,
		description TEXT,
		FOREIGN KEY (record_id) REFERENCES records(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_related_cbi ON related_records(related_cbi)`,
	`CREATE INDEX IF NOT EXISTS idx_related_record ON related_records(record_id)`,

	// Add current_custodian column to records (safe to run on existing DBs)
	`CREATE TABLE IF NOT EXISTS _migration_marker (id INTEGER PRIMARY KEY)`,
}

func (s *Store) migrate() error {
	for i, m := range migrations {
		if _, err := s.DB.Exec(m); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}
