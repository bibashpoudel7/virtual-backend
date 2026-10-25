-- Remove the tour intro 3D model
ALTER TABLE tours DROP COLUMN IF EXISTS intro_model_url;
