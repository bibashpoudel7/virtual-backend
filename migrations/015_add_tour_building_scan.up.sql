-- Room layout measured with the Android capture app (ARCore), used by the dollhouse builder
ALTER TABLE tours ADD COLUMN IF NOT EXISTS building_scan jsonb;
