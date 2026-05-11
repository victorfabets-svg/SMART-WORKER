# SMART_WORKER

SMART_WORKER is an AI Operational Memory System (AOMS).

Its purpose is to provide persistent operational memory, contextual retrieval, architectural knowledge, and shared cognition infrastructure for AI agents working on software projects.

This repository is not a product application.

It is a cognitive infrastructure layer designed to support:
- OpenHands
- Claude Code
- Codex
- Cursor
- autonomous coding agents
- multi-agent systems

---

# Core Objectives

- Persistent operational memory
- Shared project cognition
- Semantic retrieval
- Runtime context generation
- Architecture decision storage
- Coding standards persistence
- Conversation ingestion
- Context orchestration

---

# Planned Stack

## Backend
- Go

## Database
- PostgreSQL
- pgvector
- Supabase

## AI Infrastructure
- embeddings
- retrieval pipelines
- context builders
- summarizers

---

# Initial Architecture

```txt
SMART_WORKER/
│
├── cmd/
├── internal/
├── memory/
├── migrations/
├── scripts/
├── docs/
└── api/
```

---

# Philosophy

The system must remain:

- modular
- scalable
- infrastructure-focused
- operational
- minimal
- extensible
- cleanly architected

Avoid:
- overengineering
- unnecessary abstractions
- fake business logic
- framework bloat

---

# Future Roadmap

## Phase 1
Infrastructure foundation

## Phase 2
Memory ingestion pipeline

## Phase 3
Semantic retrieval engine

## Phase 4
Runtime context assembly

## Phase 5
Multi-agent orchestration

---

# Getting Started

Copy environment configuration:

```bash
cp .env.example .env
```

Start database with Docker:

```bash
docker-compose up -d postgres
```

Run the server:

```bash
make run
```

---

# Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `SUPABASE_URL` | Supabase project URL |
| `SUPABASE_ANON_KEY` | Supabase anonymous key |
| `SUPABASE_SERVICE_ROLE` | Supabase service role key |
| `OPENAI_API_KEY` | OpenAI API key for embeddings |
| `PORT` | Server port (default: 8080) |
| `LOG_LEVEL` | Logging level (default: info) |

---

# Interfaces

Key interfaces defined (to be implemented in Phase 2):

- `MemoryRepository` - PDR, ADR, preferences storage
- `EmbeddingProvider` - Text embedding generation
- `RetrievalEngine` - Semantic search
- `ContextBuilder` - Runtime context assembly
- `ConversationIngestion` - Conversation storage
- `RuntimeAssembler` - Runtime context

---

# Status

Project initialization phase.

Architecture skeleton complete.

Ready for Phase 2: Core implementations.
