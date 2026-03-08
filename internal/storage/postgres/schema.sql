-- Cities Social Simulator — PostgreSQL Schema (v2)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Players ────────────────────────────────────────────────────────────────

CREATE TABLE players (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    city_id       UUID NOT NULL,
    session_token TEXT UNIQUE NOT NULL,
    secret_key    TEXT NOT NULL,   -- HMAC-SHA256 key for message signing
    connected_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Cities ─────────────────────────────────────────────────────────────────

CREATE TABLE cities (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    mayor_id      UUID NOT NULL,
    mayor_name    TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','vacation','collapsing','ruins')),
    population    INT NOT NULL DEFAULT 500,
    happiness     FLOAT NOT NULL DEFAULT 60.0,
    treasury      BIGINT NOT NULL DEFAULT 10000,
    tax_rate      FLOAT NOT NULL DEFAULT 20.0,
    round         INT NOT NULL DEFAULT 0,
    peak_population INT NOT NULL DEFAULT 500,
    population_breakdown  JSONB NOT NULL DEFAULT '{}',
    buildings     JSONB NOT NULL DEFAULT '[]',
    products      JSONB NOT NULL DEFAULT '[]',
    trade_routes  JSONB NOT NULL DEFAULT '[]',
    stats         JSONB NOT NULL DEFAULT '{}',
    active_policies JSONB NOT NULL DEFAULT '[]',
    vacation_data JSONB,         -- VacationMode struct
    ruins_data    JSONB,         -- RuinsData struct (when status='ruins')
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cities_mayor ON cities(mayor_id);
CREATE INDEX idx_cities_status ON cities(status);

-- ─── Heartbeats ─────────────────────────────────────────────────────────────

CREATE TABLE heartbeats (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    city_id    UUID NOT NULL REFERENCES cities(id),
    round      INT NOT NULL,
    proposals  JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_heartbeats_city ON heartbeats(city_id);
CREATE INDEX idx_heartbeats_round ON heartbeats(round);

-- ─── Mayor Decisions ─────────────────────────────────────────────────────────

CREATE TABLE decisions (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    heartbeat_id   UUID NOT NULL REFERENCES heartbeats(id),
    city_id        UUID NOT NULL REFERENCES cities(id),
    player_id      UUID NOT NULL,
    initiative_id  TEXT NOT NULL,
    decided_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_decisions_city ON decisions(city_id);

-- ─── Trade Orders ─────────────────────────────────────────────────────────

CREATE TABLE trade_orders (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_city_id   UUID NOT NULL REFERENCES cities(id),
    to_city_id     UUID NOT NULL REFERENCES cities(id),
    product_id     TEXT NOT NULL,
    product_tier   INT NOT NULL DEFAULT 1,
    quantity       INT NOT NULL,
    price_per_unit BIGINT NOT NULL,
    direction      TEXT NOT NULL CHECK (direction IN ('sell', 'buy')),
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','settled','cancelled')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at     TIMESTAMPTZ
);

CREATE INDEX idx_trade_from ON trade_orders(from_city_id);
CREATE INDEX idx_trade_to ON trade_orders(to_city_id);
CREATE INDEX idx_trade_status ON trade_orders(status);

-- ─── Global Refugee Pool Log ─────────────────────────────────────────────────

CREATE TABLE refugee_pool_log (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    round      INT NOT NULL,
    delta      INT NOT NULL,       -- positive=added, negative=absorbed
    total      INT NOT NULL,       -- pool size after change
    reason     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Population Events ─────────────────────────────────────────────────────

CREATE TABLE population_events (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    city_id    UUID NOT NULL REFERENCES cities(id),
    round      INT NOT NULL,
    event_type TEXT NOT NULL,    -- ARRIVED, LEFT, BORN, PROMOTED
    group_name TEXT NOT NULL,
    count      INT NOT NULL,
    reason     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pop_events_city_round ON population_events(city_id, round);

-- ─── World Events ─────────────────────────────────────────────────────────

CREATE TABLE world_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type  TEXT NOT NULL,   -- HEARTBEAT_COMPLETE, CITY_COLLAPSED, CITY_FOUNDED, etc.
    description TEXT NOT NULL,
    city_id     UUID REFERENCES cities(id),
    city_name   TEXT,
    round       INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── City Snapshots (history/charting) ─────────────────────────────────────

CREATE TABLE city_snapshots (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    city_id     UUID NOT NULL REFERENCES cities(id),
    round       INT NOT NULL,
    population  INT NOT NULL,
    happiness   FLOAT NOT NULL,
    treasury    BIGINT NOT NULL,
    tax_rate    FLOAT NOT NULL,
    gdp         BIGINT NOT NULL DEFAULT 0,
    pollution   INT NOT NULL DEFAULT 0,
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_snapshots_city_round ON city_snapshots(city_id, round);

-- ─── City Recovery Transactions ──────────────────────────────────────────────

CREATE TABLE city_recoveries (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ruins_city_id  UUID NOT NULL REFERENCES cities(id),
    new_mayor_id   UUID NOT NULL,
    new_mayor_name TEXT NOT NULL,
    recovery_cost  BIGINT NOT NULL,
    paid_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Views ──────────────────────────────────────────────────────────────────

CREATE VIEW city_leaderboard AS
SELECT
    c.id,
    c.name,
    c.mayor_name,
    c.status,
    c.population,
    c.happiness,
    c.treasury,
    c.peak_population,
    (c.stats->>'gdp')::BIGINT         AS gdp,
    (c.stats->>'innovation_index')::INT AS innovation,
    (c.stats->>'pollution_level')::INT  AS pollution,
    c.round
FROM cities c
ORDER BY c.happiness DESC, c.population DESC;

CREATE VIEW refugee_pool_current AS
SELECT COALESCE(SUM(delta), 0) AS total_refugees
FROM refugee_pool_log;
