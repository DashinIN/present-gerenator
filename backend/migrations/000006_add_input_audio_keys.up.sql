ALTER TABLE generation_requests
    ADD COLUMN input_audio_keys TEXT[] NOT NULL DEFAULT '{}';

UPDATE generation_requests
SET input_audio_keys = CASE
    WHEN input_audio_key = '' THEN '{}'
    ELSE ARRAY[input_audio_key]
END
WHERE input_audio_keys = '{}';
