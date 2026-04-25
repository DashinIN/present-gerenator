ALTER TABLE tariffs ADD COLUMN IF NOT EXISTS price_per_lyrics INT NOT NULL DEFAULT 2;
UPDATE tariffs SET price_per_lyrics = 2 WHERE is_active = true;
