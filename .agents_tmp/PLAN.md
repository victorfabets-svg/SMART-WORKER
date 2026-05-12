# Telegram Runtime Import Resolution Audit & Correction Plan

## 1. CANONICAL ROOT MODULE NAME

**Exact module path:** `github.com/aoms/smart-worker`

**Expected import prefix:** `github.com/aoms/smart-worker/`

## 2. INVALID IMPORT INVENTORY

### CRITICAL ISSUE FOUND:

| File | Line | Invalid Import | Expected Import |
|------|------|----------------|-----------------|
| `integrations/telegram/cmd/bot/main.go` | 14 | `"SMART-WORKER/integrations/telegram/context"` | `"github.com/aoms/smart-worker/integrations/telegram/context"` |

### Root Cause Analysis:
- The import uses legacy uppercase `SMART-WORKER` which does not resolve in Go
- Go module path expects lowercase: `github.com/aoms/smart-worker`
- This is the root cause of the reported failure: `package SMART-WORKER/integrations/telegram/context is not in std`

## 3. AFFECTED FILES

### Files Requiring Correction:

1. **PRIMARY:** `/workspace/project/SMART-WORKER/integrations/telegram/cmd/bot/main.go` — Line 14 (fix import)
2. **DELETE:** `/workspace/project/SMART-WORKER/integrations/telegram/cmd/bot/go.mod` — nested module
3. **DELETE:** `/workspace/project/SMART-WORKER/integrations/telegram/runtime/go.mod` — nested module

### Root Module (Already Correct):
- `/workspace/project/SMART-WORKER/go.mod` — Module: `github.com/aoms/smart-worker`

## 4. DEPENDENCY GRAPH

```
github.com/aoms/smart-worker (root module)
├── cmd/api/main.go
│   └── imports: internal/{config,database,embeddings,logger,memory}
│
└── integrations/telegram/
    ├── context/memory.go (standalone)
    ├── cmd/bot/main.go 
    │   └── imports: telegram context + external bot API
    └── runtime/main.go (standalone)
```

## 5. REMAINING CONTAMINATION MAP

### Must Delete:
1. `/workspace/project/SMART-WORKER/integrations/telegram/cmd/bot/go.mod` - legacy module
2. `/workspace/project/SMART-WORKER/integrations/telegram/runtime/go.mod` - nested module

### Runtime Paths (Acceptable):
- Files reference `/workspace/project/SMART-WORKER` in exec paths - this is container path, NOT import path

## 6. SAFE CORRECTION STRATEGY

### Step 1: Fix Invalid Import
**File:** `integrations/telegram/cmd/bot/main.go`
**Line 14:** Change from:
```go
"SMART-WORKER/integrations/telegram/context"
```
To:
```go
"github.com/aoms/smart-worker/integrations/telegram/context"
```

### Step 2: Delete Nested Modules
1. Remove: `/workspace/project/SMART-WORKER/integrations/telegram/cmd/bot/go.mod`
2. Remove: `/workspace/project/SMART-WORKER/integrations/telegram/runtime/go.mod`

### Step 3: Validate Build
```bash
cd /workspace/project/SMART-WORKER && go build ./...
```

## 7. RISK ANALYSIS

| Risk | Probability | Impact |
|------|-------------|--------|
| Import path typo | LOW | Build failure - verifyable |
| Deleting nested go.mod | LOW | Scripts use bash, not Go modules |

**Blast Radius:** 1 file edit + 2 file deletions

## 8. VALIDATION STRATEGY

### Post-Execution Validation:
```bash
# Build all
cd /workspace/project/SMART-WORKER && go build ./...

# Verify telegram packages
go list ./integrations/telegram/...

# Verify single module
find . -name "go.mod" | wc -l
# Expected: 1
```

### Success Criteria:
- `go build ./...` completes without errors
- No `SMART-WORKER` uppercase references in imports
- Only single root `go.mod` exists

## 9. FINAL RUNTIME COMMANDS

### Build API Server:
```bash
cd /workspace/project/SMART-WORKER && go build -o aoms ./cmd/api && ./aoms
```

### Run Telegram Bot:
```bash
cd /workspace/project/SMART-WORKER && go run ./integrations/telegram/cmd/bot
```

### Run Telegram Runtime:
```bash
cd /workspace/project/SMART-WORKER && go run ./integrations/telegram/runtime
```

## SUMMARY

| Item | Count |
|------|-------|
| Invalid imports to fix | 1 |
| Files requiring edit | 1 |
| Nested modules to delete | 2 |

**Execution order:**
1. Fix import in `cmd/bot/main.go` line 14
2. Delete nested `go.mod` files
3. Validate with `go build ./...`

**Expected outcome:** All Go packages compile under single root module
