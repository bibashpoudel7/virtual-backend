-- Remove the measured building scan
ALTER TABLE tours DROP COLUMN IF EXISTS building_scan;
