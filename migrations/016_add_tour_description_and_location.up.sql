-- Add description and location to tours for SEO and property details
ALTER TABLE tours ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE tours ADD COLUMN IF NOT EXISTS location varchar(255);
