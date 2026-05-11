# BEATSET Executor Agent

## Purpose

Receive structured findings from the Auditor Agent and implement deterministic, surgical fixes. The Executor operates as a targeted code modifier that applies fixes based on verified evidence, preserving architectural consistency and operational stability.

## Execution Flow

1. **Receive Finding**: Load finding from file or memory endpoint
2. **Bootstrap Context**: Fetch operational context from SMART-WORKER
3. **Analyze Root Cause**: Identify exact file(s) and line(s) requiring changes
4. **Apply Targeted Fix**: Modify ONLY files relevant to the finding
5. **Validate Changes**: Verify syntax, compilation, and basic correctness
6. **Generate Commit**: Create safe commit with diagnostic message

## SMART-WORKER Integration

- **Target Repository**: `/workspaces/BEATSET` (configurable)
- **Memory Endpoint**: `http://localhost:8080/memory/ingest`
- **Bootstrap Endpoint**: `http://localhost:8080/agent/bootstrap`
- **Findings Input**: Via file or API

## Safety Boundaries

- ✅ Modify files only for the specific finding
- ✅ Preserve existing test coverage
- ✅ Avoid touching unrelated systems
- ✅ Avoid broad refactoring
- ✅ Minimize surface area changes

- ❌ NOT autonomous AGI
- ❌ NOT self-improving system
- ❌ NOT auto-deployer
- ❌ NOT infrastructure healer

## Commit Behavior

- One commit per fix
- Include finding title in commit message
- Include evidence reference
- Co-authored-by header for AI attribution

## Usage

```bash
cd agents/executor

# Run with finding file
FINDING_FILE=/path/to/finding.json ./run.sh

# Override target via environment
TARGET_REPO=/custom/path ./run.sh
```