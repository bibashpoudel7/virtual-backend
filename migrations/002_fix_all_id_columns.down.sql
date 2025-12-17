-- Rollback Migration: Revert ID columns back to VARCHAR(26)
-- WARNING: This will fail if any existing data exceeds 26 characters

-- Revert foreign key columns first
ALTER TABLE overlays ALTER COLUMN scene_id TYPE VARCHAR(26);
ALTER TABLE overlays ALTER COLUMN tour_id TYPE VARCHAR(26);
ALTER TABLE hotspots ALTER COLUMN scene_id TYPE VARCHAR(26);
ALTER TABLE hotspots ALTER COLUMN tour_id TYPE VARCHAR(26);
ALTER TABLE tour_scenes ALTER COLUMN scene_id TYPE VARCHAR(26);
ALTER TABLE tour_scenes ALTER COLUMN tour_id TYPE VARCHAR(26);
ALTER TABLE scenes ALTER COLUMN tour_id TYPE VARCHAR(26);

-- Revert primary key columns
ALTER TABLE overlays ALTER COLUMN id TYPE VARCHAR(26);
ALTER TABLE hotspots ALTER COLUMN id TYPE VARCHAR(26);
ALTER TABLE scenes ALTER COLUMN id TYPE VARCHAR(26);
ALTER TABLE tour_scenes ALTER COLUMN id TYPE VARCHAR(26);
ALTER TABLE tours ALTER COLUMN id TYPE VARCHAR(26);