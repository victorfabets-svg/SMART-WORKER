#!/bin/bash
# Run database migrations

set -e

DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/aoms?sslmode=disable}"

echo "Running migrations..."

for f in migrations/*.sql; do
    echo "Applying $f"
    psql "$DATABASE_URL" -f "$f"
done

echo "Migrations complete"