CREATE TABLE margin_active_positions (
    id SERIAL PRIMARY KEY,
    user_address VARCHAR(42) NOT NULL,
    market_id SMALLINT NOT NULL,
    has_collateral BOOLEAN DEFAULT FALSE NOT NULL,
    has_debt BOOLEAN DEFAULT FALSE NOT NULL,
    is_active BOOLEAN DEFAULT FALSE NOT NULL, -- Should be TRUE if has_collateral OR has_debt
    last_activity_timestamp BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE (user_address, market_id)
);

CREATE INDEX idx_margin_active_positions_user_market ON margin_active_positions(user_address, market_id);
CREATE INDEX idx_margin_active_positions_is_active ON margin_active_positions(is_active);
