ALTER TABLE generation_requests
    DROP COLUMN IF EXISTS recipient_name,
    DROP COLUMN IF EXISTS occasion;
