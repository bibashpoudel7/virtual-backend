-- Remove intro model room labels
ALTER TABLE tours DROP COLUMN IF EXISTS intro_model_rooms;
