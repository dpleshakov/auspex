-- Eve location names cache (stations and structures, populated lazily on first encounter)
CREATE TABLE eve_locations (
    id          INTEGER PRIMARY KEY,  -- EVE location_id (station or structure)
    name        TEXT NOT NULL,
    resolved_at DATETIME NOT NULL     -- last successful resolution timestamp
);
