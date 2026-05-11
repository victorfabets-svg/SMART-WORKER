# API Documentation

This directory will contain API specifications.

## Endpoints (planned)

### Memory

- `GET /api/v1/pdr` - List PDRs
- `POST /api/v1/pdr` - Create PDR
- `GET /api/v1/pdr/:id` - Get PDR
- `GET /api/v1/adr` - List ADRs
- `POST /api/v1/adr` - Create ADR
- `GET /api/v1/adr/:id` - Get ADR
- `GET /api/v1/preferences` - List preferences
- `POST /api/v1/preferences` - Create preference

### Retrieval

- `GET /api/v1/search` - Semantic search
- `POST /api/v1/search` - Search with filters

### Runtime

- `POST /api/v1/context` - Build runtime context
- `GET /api/v1/runtime/:agentId` - Get runtime
- `POST /api/v1/runtime/refresh` - Refresh runtime

### Ingestion

- `POST /api/v1/conversations` - Ingest conversation
- `GET /api/v1/conversations` - List conversations