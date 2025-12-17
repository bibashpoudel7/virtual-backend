-- Migration: Fix ALL ID columns to accommodate UUIDs (36 chars)
-- This migration updates all ID columns from VARCHAR(26) to VARCHAR(50)

-- Update primary key columns
ALTER TABLE tours ALTER COLUMN id TYPE VARCHAR(50);
ALTER TABLE tour_scenes ALTER COLUMN id TYPE VARCHAR(50);
ALTER TABLE scenes ALTER COLUMN id TYPE VARCHAR(50);
ALTER TABLE hotspots ALTER COLUMN id TYPE VARCHAR(50);
ALTER TABLE overlays ALTER COLUMN id TYPE VARCHAR(50);

-- Update foreign key columns that reference these IDs
ALTER TABLE scenes ALTER COLUMN tour_id TYPE VARCHAR(50);
ALTER TABLE tour_scenes ALTER COLUMN tour_id TYPE VARCHAR(50);
ALTER TABLE tour_scenes ALTER COLUMN scene_id TYPE VARCHAR(50);
ALTER TABLE hotspots ALTER COLUMN tour_id TYPE VARCHAR(50);
ALTER TABLE hotspots ALTER COLUMN scene_id TYPE VARCHAR(50);
ALTER TABLE overlays ALTER COLUMN tour_id TYPE VARCHAR(50);
ALTER TABLE overlays ALTER COLUMN scene_id TYPE VARCHAR(50);

-- Update payment_id if it exists and is VARCHAR(26)
-- (commented out as it appears to be TEXT type already)