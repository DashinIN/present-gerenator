ALTER TABLE generation_requests DROP COLUMN IF EXISTS parent_id;
ALTER TABLE generation_requests DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS generation_sessions;
