-- EVE universe reference data (populated lazily on first encounter)
CREATE TABLE eve_categories (
    id    INTEGER PRIMARY KEY,  -- EVE category_id
    name  TEXT NOT NULL
);

CREATE TABLE eve_groups (
    id          INTEGER PRIMARY KEY,  -- EVE group_id
    category_id INTEGER NOT NULL REFERENCES eve_categories(id),
    name        TEXT NOT NULL
);

CREATE TABLE eve_types (
    id       INTEGER PRIMARY KEY,  -- EVE type_id
    group_id INTEGER NOT NULL REFERENCES eve_groups(id),
    name     TEXT NOT NULL
);

-- Authorized characters (one OAuth token per character)
CREATE TABLE characters (
    id            INTEGER PRIMARY KEY,  -- EVE character_id
    name          TEXT NOT NULL,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expiry  DATETIME NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Tracked corporations (accessed via delegate character)
CREATE TABLE corporations (
    id           INTEGER PRIMARY KEY,  -- EVE corporation_id
    name         TEXT NOT NULL,
    delegate_id  INTEGER NOT NULL REFERENCES characters(id),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- BPO library (all characters + corporations combined)
CREATE TABLE blueprints (
    id          INTEGER PRIMARY KEY,  -- EVE item_id
    owner_type  TEXT NOT NULL,        -- 'character' | 'corporation'
    owner_id    INTEGER NOT NULL,
    type_id     INTEGER NOT NULL REFERENCES eve_types(id),
    location_id INTEGER NOT NULL,
    me_level    INTEGER NOT NULL DEFAULT 0,
    te_level    INTEGER NOT NULL DEFAULT 0,
    updated_at  DATETIME NOT NULL
);

-- Active and ready research jobs
CREATE TABLE jobs (
    id           INTEGER PRIMARY KEY,  -- EVE job_id
    blueprint_id INTEGER NOT NULL REFERENCES blueprints(id),
    owner_type   TEXT NOT NULL,        -- 'character' | 'corporation'
    owner_id     INTEGER NOT NULL,
    installer_id INTEGER NOT NULL,     -- character_id who started the job
    activity     TEXT NOT NULL,        -- 'me_research' | 'te_research' | 'copying'
    status       TEXT NOT NULL,        -- 'active' | 'ready'
    start_date   DATETIME NOT NULL,
    end_date     DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

-- ESI cache state per subject per endpoint
CREATE TABLE sync_state (
    owner_type  TEXT NOT NULL,
    owner_id    INTEGER NOT NULL,
    endpoint    TEXT NOT NULL,      -- 'blueprints' | 'jobs'
    last_sync   DATETIME NOT NULL,
    cache_until DATETIME NOT NULL,
    PRIMARY KEY (owner_type, owner_id, endpoint)
);
