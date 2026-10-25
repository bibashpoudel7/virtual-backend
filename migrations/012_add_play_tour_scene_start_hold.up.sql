-- How long a play tour step stays on its start view before moving (ms)
ALTER TABLE play_tour_scenes ADD COLUMN IF NOT EXISTS start_hold bigint DEFAULT 0;
