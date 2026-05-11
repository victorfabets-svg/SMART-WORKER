# BEATSET Auditor Agent

## Purpose

Continuously inspect the BEATSET repository to identify operational failures, bugs, regressions, inconsistencies, dead code, architecture violations, and runtime risks. Generate structured findings and ingest them into SMART-WORKER memory.

## Scope

- **Repository Inspection**: Go runtime, frontend architecture, directory structure
- **Code Quality**: Dead code, duplicated logic, performance risks
- **Infrastructure**: CI/CD pipelines, configuration
- **Production Risks**: Runtime failures, security issues, operational inconsistencies

## Execution Flow

1. Bootstrap runtime context from SMART-WORKER
2. Load system prompt from `prompt.md`
3. Execute audit cycle by analyzing BEATSET codebase
4. Generate structured findings in JSON format
5. Save findings to SMART-WORKER memory endpoint

## Expected Outputs

Structured findings in JSON format:

```json
{
  "type": "bug|dead_code|risk|violation",
  "severity": "critical|high|medium|low",
  "title": "Brief title",
  "content": "Detailed description",
  "evidence": "File paths, line numbers, code snippets",
  "recommended_fix": "Actionable fix recommendation"
}
```

## SMART-WORKER Integration

- **Target Repository**: Configured via `config.json` or `TARGET_REPO` env var
- **Memory Endpoint**: `http://localhost:8080/memory/ingest`
- **Bootstrap Endpoint**: `http://localhost:8080/agent/bootstrap`
- **Config**: See `config.json`

## Usage

```bash
# Default - uses config.json target
cd agents/auditor
./run.sh

# Override target via environment
TARGET_REPO=/path/to/repo ./run.sh
```

## Constraints

- Read-only operation (no code modification)
- Maximum 20 findings per run
- No Docker, Redis, RabbitMQ, or orchestration frameworks
- Single operational auditor (no swarm/mesh)