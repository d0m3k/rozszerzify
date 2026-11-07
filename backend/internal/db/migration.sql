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

-- Per-user dates (multi-account support). NULL → the instance-wide
-- START_DATE / BIRTH_DATE env values are used as fallback (legacy).
ALTER TABLE rz_users ADD COLUMN IF NOT EXISTS birth_date DATE;
ALTER TABLE rz_users ADD COLUMN IF NOT EXISTS start_date DATE;

-- Categories are per account and user-editable (a food can belong to several,
-- e.g. jajko is both an "alergeny" and a "mięso i ryby"-ish protein). The
-- built-in list is seeded per user by EnsureCategories on boot.
CREATE TABLE IF NOT EXISTS rz_categories (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES rz_users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    emoji      TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 500,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);

-- Membership: rz_foods.category stays the *primary* category (the first one),
-- this table holds every category the food belongs to, primary included.
CREATE TABLE IF NOT EXISTS rz_food_categories (
    food_id     INTEGER NOT NULL REFERENCES rz_foods(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES rz_categories(id) ON DELETE CASCADE,
    PRIMARY KEY (food_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_rz_categories_user ON rz_categories (user_id, sort_order, name);
CREATE INDEX IF NOT EXISTS idx_rz_food_categories_cat ON rz_food_categories (category_id);

-- Per-food icon; empty → the frontend falls back to its name table, then to
-- the primary category's emoji.
ALTER TABLE rz_foods ADD COLUMN IF NOT EXISTS emoji TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_rz_food_log_food ON rz_food_log (food_id, id DESC);
-- "What did he eat last" is ordered by when the meal happened, not when it was
-- typed in (tries can be backdated), hence this second index.
CREATE INDEX IF NOT EXISTS idx_rz_food_log_food_tried ON rz_food_log (food_id, tried_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_rz_food_log_user_day ON rz_food_log (user_id, tried_at);
-- A meal (danie/posiłek) groups several existing foods. Eating the meal bumps
-- the meal's own try counter AND records one try for every food it contains.
CREATE TABLE IF NOT EXISTS rz_meals (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES rz_users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    emoji         TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    tries         INTEGER NOT NULL DEFAULT 0,
    target        INTEGER NOT NULL DEFAULT 15,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_tried_at TIMESTAMPTZ,
    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_rz_meals_user ON rz_meals (user_id, name);

-- Composition of a meal, ordered like it is served. Deleting a food simply
-- drops it from every meal that used it.
CREATE TABLE IF NOT EXISTS rz_meal_items (
    meal_id  INTEGER NOT NULL REFERENCES rz_meals(id) ON DELETE CASCADE,
    food_id  INTEGER NOT NULL REFERENCES rz_foods(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (meal_id, food_id)
);

CREATE INDEX IF NOT EXISTS idx_rz_meal_items_food ON rz_meal_items (food_id);

-- One event per eaten meal (the parent tap). Its item tries live in
-- rz_food_log and point back here, so a food history can tell the parent
-- "this try was part of meal X".
CREATE TABLE IF NOT EXISTS rz_meal_log (
    id       SERIAL PRIMARY KEY,
    meal_id  INTEGER NOT NULL REFERENCES rz_meals(id) ON DELETE CASCADE,
    user_id  INTEGER NOT NULL REFERENCES rz_users(id) ON DELETE CASCADE,
    note     TEXT NOT NULL DEFAULT '',
    rating   INTEGER NOT NULL DEFAULT 3,
    tried_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rz_meal_log_meal ON rz_meal_log (meal_id, tried_at DESC, id DESC);

-- Food tries that were generated by a meal. meal_log_id links back to the
-- parent event (NULL after that event/meal is deleted); meal_name is kept
-- denormalized so the food history still explains how the try happened.
ALTER TABLE rz_food_log ADD COLUMN IF NOT EXISTS meal_log_id INTEGER REFERENCES rz_meal_log(id) ON DELETE SET NULL;
ALTER TABLE rz_food_log ADD COLUMN IF NOT EXISTS meal_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_rz_food_log_meal ON rz_food_log (meal_log_id);
