-- Rollback: Remove auto-change functionality fields

-- Remove indexes
DROP INDEX IF EXISTS idx_tours_auto_change;
DROP INDEX IF EXISTS idx_scenes_tour_order;

-- Remove columns from scenes table
ALTER TABLE scenes
DROP COLUMN IF EXISTS duration,
DROP COLUMN IF EXISTS transition_type,
DROP COLUMN IF EXISTS transition_duration,
DROP COLUMN IF EXISTS auto_rotate,
DROP COLUMN IF EXISTS auto_rotate_speed;

-- Remove columns from tours table
ALTER TABLE tours
DROP COLUMN IF EXISTS auto_change_enabled,
DROP COLUMN IF EXISTS auto_change_interval,
DROP COLUMN IF EXISTS auto_change_mode,
DROP COLUMN IF EXISTS auto_pause_on_interaction,
DROP COLUMN IF EXISTS auto_restart_delay;