-- Corp assets cache (populated during syncCorpAssets, used for CorpSAG location resolution)
CREATE TABLE corp_assets (
    item_id       INTEGER PRIMARY KEY,  -- Corporation Office Item ID (= location_id in corp blueprints)
    owner_id      INTEGER NOT NULL,     -- corporation_id
    location_id   INTEGER NOT NULL,     -- real station or structure ID
    location_type TEXT NOT NULL
);

-- Track the ESI location_flag per blueprint for accurate location resolution
ALTER TABLE blueprints ADD COLUMN location_flag TEXT NOT NULL DEFAULT '';
