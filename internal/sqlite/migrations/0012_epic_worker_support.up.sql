ALTER TABLE epic ADD COLUMN claimed_at DATETIME;
ALTER TABLE epic ADD COLUMN last_heartbeat_at DATETIME;
ALTER TABLE epic ADD COLUMN feedback TEXT;
ALTER TABLE epic ADD COLUMN feedback_type TEXT CHECK(feedback_type IN ('message', 'confirmed', 'closed'));
