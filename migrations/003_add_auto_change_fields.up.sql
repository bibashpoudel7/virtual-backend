-- Migration: Add auto-change functionality fields to tours and scenes
-- This enables automatic scene transitions in virtual tours

-- Add auto-change fields to tours table
ALTER TABLE tours 
ADD COLUMN IF NOT EXISTS auto_change_enabled BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS auto_change_interval INTEGER DEFAULT 5000,
ADD COLUMN IF NOT EXISTS auto_change_mode VARCHAR(20) DEFAULT 'sequential',
ADD COLUMN IF NOT EXISTS auto_pause_on_interaction BOOLEAN DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS auto_restart_delay INTEGER DEFAULT 30000;

-- Add auto-change and transition fields to scenes table
ALTER TABLE scenes
ADD COLUMN IF NOT EXISTS duration INTEGER,
ADD COLUMN IF NOT EXISTS transition_type VARCHAR(20) DEFAULT 'fade',
ADD COLUMN IF NOT EXISTS transition_duration INTEGER DEFAULT 1000,
ADD COLUMN IF NOT EXISTS auto_rotate BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS auto_rotate_speed NUMERIC DEFAULT 0.5;

-- Add indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_tours_auto_change ON tours(auto_change_enabled);
CREATE INDEX IF NOT EXISTS idx_scenes_tour_order ON scenes(tour_id, scene_order);