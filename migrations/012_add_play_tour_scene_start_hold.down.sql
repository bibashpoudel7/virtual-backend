-- Remove start pause from play tour steps
ALTER TABLE play_tour_scenes DROP COLUMN IF EXISTS start_hold;
