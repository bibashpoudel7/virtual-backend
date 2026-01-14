-- Remove is_featured_on_homepage column from tours table
DROP INDEX IF EXISTS idx_tours_featured;
ALTER TABLE tours DROP COLUMN IF EXISTS is_featured_on_homepage;
