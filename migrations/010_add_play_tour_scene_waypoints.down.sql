-- Remove curved camera path points from play tour scenes
ALTER TABLE play_tour_scenes DROP COLUMN IF EXISTS waypoints;
