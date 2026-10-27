-- Migration rollback: Remove background_audio_url column from tours table

ALTER TABLE tours 
DROP COLUMN IF EXISTS background_audio_url;