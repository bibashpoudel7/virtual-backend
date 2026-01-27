-- Remove categories column from tours table
ALTER TABLE tours DROP COLUMN IF EXISTS categories;