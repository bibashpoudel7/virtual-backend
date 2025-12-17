-- Migration: Fix user_id column size to accommodate UUIDs (36 chars)
-- Previous: VARCHAR(26) - ULID format
-- New: VARCHAR(50) - UUID format with extra space

-- Update tours table
ALTER TABLE tours ALTER COLUMN user_id TYPE VARCHAR(50);

-- Update any other tables that reference user_id with VARCHAR(26)
-- Note: Adjust table names based on your actual schema

-- If there are foreign key constraints, you may need to update those as well
-- Example for other potential tables (uncomment if they exist):
-- ALTER TABLE tour_scenes ALTER COLUMN user_id TYPE VARCHAR(50);
-- ALTER TABLE scenes ALTER COLUMN user_id TYPE VARCHAR(50);
-- ALTER TABLE hotspots ALTER COLUMN user_id TYPE VARCHAR(50);
-- ALTER TABLE overlays ALTER COLUMN user_id TYPE VARCHAR(50);

-- Also update created_by and updated_by columns if they exist and have the same issue
ALTER TABLE tours ALTER COLUMN created_by TYPE VARCHAR(50);
ALTER TABLE tours ALTER COLUMN updated_by TYPE VARCHAR(50);