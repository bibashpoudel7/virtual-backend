-- Clickable room labels placed on the tour's intro 3D model
ALTER TABLE tours ADD COLUMN IF NOT EXISTS intro_model_rooms jsonb;
