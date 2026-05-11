-- Create architecture_decisions table
-- Stores Architecture Decision Records

CREATE TABLE IF NOT EXISTS architecture_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    context TEXT NOT NULL,
    decision TEXT NOT NULL,
    consequences TEXT,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_adr_created_at ON architecture_decisions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_adr_title ON architecture_decisions(title);
CREATE INDEX IF NOT EXISTS idx_adr_tags ON architecture_decisions USING GIN(tags);