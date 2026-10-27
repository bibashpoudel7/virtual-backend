-- Optional curved camera path points for play tour scenes
ALTER TABLE play_tour_scenes ADD COLUMN IF NOT EXISTS waypoints jsonb;
