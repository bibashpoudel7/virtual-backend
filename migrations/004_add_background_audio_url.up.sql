-- Migration: Add background_audio_url column to tours table
-- This enables background audio functionality for virtual tours

ALTER TABLE tours 
ADD COLUMN IF NOT EXISTS background_audio_url VARCHAR(500);