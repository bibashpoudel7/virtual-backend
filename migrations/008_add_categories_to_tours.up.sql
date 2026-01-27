-- Add categories column as text array to tours table
ALTER TABLE tours ADD COLUMN categories text[] DEFAULT '{}';