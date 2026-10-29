-- Add title and description columns to play_tour_scenes table
ALTER TABLE play_tour_scenes ADD COLUMN IF NOT EXISTS title TEXT;
ALTER TABLE play_tour_scenes ADD COLUMN IF NOT EXISTS description TEXT;
