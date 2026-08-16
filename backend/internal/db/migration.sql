-- Rozszerzify database schema
-- Lives in the same mikr.us database as rybaspotting; all tables are
-- prefixed rz_ to avoid collisions. Idempotent: CREATE TABLE IF NOT EXISTS.

CREATE TABLE IF NOT EXISTS rz_users (
    id            SERIAL PRIMARY KEY,
    username      VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One food the kid is (or was) trying. tries counts how many times
-- the food has been offered. target is the "we can judge if he likes
-- it" threshold (classic: 15).
CREATE TABLE IF NOT EXISTS rz_foods (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES rz_users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    category      TEXT NOT NULL DEFAULT 'inne',
    tries         INTEGER NOT NULL DEFAULT 0,
    target        INTEGER NOT NULL DEFAULT 15,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_tried_at TIMESTAMPTZ,
    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_rz_foods_user ON rz_foods (user_id);

-- History of individual try events (every tap on "+1").
-- rating: 1 = nie smakuje, 2 = zjada ale niechętnie, 3 = smakuje, 4 = bardzo smakuje
CREATE TABLE IF NOT EXISTS rz_food_log (
    id       SERIAL PRIMARY KEY,
    food_id  INTEGER NOT NULL REFERENCES rz_foods(id) ON DELETE CASCADE,
    user_id  INTEGER NOT NULL REFERENCES rz_users(id) ON DELETE CASCADE,
    note     TEXT NOT NULL DEFAULT '',
    rating   INTEGER NOT NULL DEFAULT 3,
    tried_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Column added later; keep for idempotency if the table already existed.
ALTER TABLE rz_food_log ADD COLUMN IF NOT EXISTS rating INTEGER NOT NULL DEFAULT 3;

CREATE INDEX IF NOT EXISTS idx_rz_food_log_food ON rz_food_log (food_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_rz_food_log_user_day ON rz_food_log (user_id, tried_at);