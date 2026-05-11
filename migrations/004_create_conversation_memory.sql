-- Create conversation_memory table
-- Stores conversation history with embeddings for semantic search

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS conversation_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    summary TEXT,
    embedding vector(1536),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for vector similarity search
CREATE INDEX IF NOT EXISTS idx_cm_embedding ON conversation_memory USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_cm_created_at ON conversation_memory(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cm_source ON conversation_memory(source);