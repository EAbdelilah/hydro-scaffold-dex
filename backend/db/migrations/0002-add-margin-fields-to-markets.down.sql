ALTER TABLE markets
DROP COLUMN borrow_enable,
DROP COLUMN liquidate_rate,
DROP COLUMN withdraw_rate,
DROP COLUMN auction_ratio_start,
DROP COLUMN auction_ratio_per_block;
