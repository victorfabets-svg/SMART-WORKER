-- Create coding_preferences table
-- Stores coding standards and preferences

CREATE TABLE IF NOT EXISTS coding_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category TEXT NOT NULL,
    rule TEXT NOT NULL,
    example TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cp_created_at ON coding_preferences(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cp_category ON coding_preferences(category);