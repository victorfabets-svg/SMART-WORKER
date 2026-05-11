# BEATSET Executor Agent

## Purpose

Receive structured findings from the Auditor Agent and implement **risk-gated** fixes. The Executor applies surgical fixes only for LOW risk issues, generates remediation prompts for MEDIUM risk, and blocks HIGH/CRITICAL risk completely.

## Governance Model

### Risk Levels

| Level | Action | Example |
|-------|--------|---------|
| **LOW** | ✅ Auto-fix → Commit | lint, imports, unused variables, isolated dead code, nil safety, log consistency, minor build fixes |
| **MEDIUM** | 🔒 Generate plan → Wait | partial refactor, backend flow changes, middleware updates, performance optimization |
| **HIGH** | 🚫 Report only → Block | auth, payments, runtime core, infra, architecture, schema, mass delete |

### Execution Matrix

| Risk | Modify Code | Commit | Approval |
|------|------------|--------|----------|
| LOW | ✅ Yes | ✅ Auto | ❌ No |
| MEDIUM | ❌ No | ❌ No | ✅ Yes |
| HIGH | ❌ No | ❌ No | 🚫 Manual |

### Safety Boundaries

- ✅ Modify max 3 files only
- ✅ Preserve test coverage
- ✅ Run `go build ./...` validation

- ❌ NEVER exceeds 3 files per fix
- ❌ NEVER modifies auth/payment code
- ❌ NEVER performs mass deletion
- ❌ NOT autonomous AGI
- ❌ NOT self-improving system

## Execution Flow

1. **Receive Finding**: Load finding JSON
2. **Classify Risk**: LOW/MEDIUM/HIGH
3. **Execute**:
   - **LOW**: Apply fix → Validate → Commit
   - **MEDIUM**: Generate plan → Save → Wait
   - **HIGH**: Generate report → Block

## SMART-WORKER Integration

- **Target Repository**: `/workspaces/BEATSET`
- **Auto-fix allowed**: `config.json` → `auto_fix_allowed`
- **Approval required**: `config.json` → `approval_required`

## Usage

```bash
cd agents/executor
./run.sh -f /path/to/finding.json
```

## Operational Philosophy

> **Safety over Speed**: When in doubt, block it out.