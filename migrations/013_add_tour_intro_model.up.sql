-- Optional 3D model (.glb) shown before the 360° tour starts
ALTER TABLE tours ADD COLUMN IF NOT EXISTS intro_model_url text;
