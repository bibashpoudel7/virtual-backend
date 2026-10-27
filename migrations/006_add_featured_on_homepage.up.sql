-- Add is_featured_on_homepage column to tours table
ALTER TABLE tours ADD COLUMN is_featured_on_homepage BOOLEAN DEFAULT FALSE NOT NULL;

-- Create index for faster queries
CREATE INDEX idx_tours_featured ON tours(is_featured_on_homepage) WHERE is_featured_on_homepage = TRUE;

-- Add comment to explain the column
COMMENT ON COLUMN tours.is_featured_on_homepage IS 'Indicates if this tour should be displayed on the homepage. Only one tour should be featured at a time.';
