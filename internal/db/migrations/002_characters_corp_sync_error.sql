ALTER TABLE characters ADD COLUMN corporation_id   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE characters ADD COLUMN corporation_name TEXT    NOT NULL DEFAULT '';

ALTER TABLE sync_state ADD COLUMN last_error TEXT;
