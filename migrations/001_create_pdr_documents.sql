-- Create pdr_documents table
-- Stores Product Requirements Documents

CREATE TABLE IF NOT EXISTS pdr_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pdr_created_at ON pdr_documents(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pdr_title ON pdr_documents(title);