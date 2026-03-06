CREATE TABLE IF NOT EXISTS rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    small_blind BIGINT NOT NULL CHECK (small_blind > 0),
    big_blind BIGINT NOT NULL CHECK (big_blind > small_blind),
    min_buy_in BIGINT NOT NULL CHECK (min_buy_in > 0),
    max_buy_in BIGINT NOT NULL CHECK (max_buy_in >= min_buy_in),
    max_players INT NOT NULL DEFAULT 9 CHECK (max_players BETWEEN 2 AND 9),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tables (
    id TEXT PRIMARY KEY,
    room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    max_players INT NOT NULL DEFAULT 9 CHECK (max_players BETWEEN 2 AND 9),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tables_room_id ON tables(room_id);
CREATE INDEX IF NOT EXISTS idx_tables_active ON tables(is_active);

CREATE TABLE IF NOT EXISTS wallets (
    player_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance BIGINT NOT NULL DEFAULT 10000 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS buy_ins (
    id BIGSERIAL PRIMARY KEY,
    player_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    table_id TEXT NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
    room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL CHECK (amount > 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'consumed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_buy_ins_player_table_status ON buy_ins(player_id, table_id, status);
CREATE INDEX IF NOT EXISTS idx_buy_ins_created_at ON buy_ins(created_at DESC);
