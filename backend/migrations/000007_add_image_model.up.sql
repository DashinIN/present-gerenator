ALTER TABLE generation_requests
ADD COLUMN IF NOT EXISTS image_model VARCHAR(64) NOT NULL DEFAULT 'gpt-image-2';
