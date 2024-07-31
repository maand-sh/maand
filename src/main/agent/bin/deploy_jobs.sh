#!/bin/bash
set -ueo pipefail

if [ -z "$(ls -A /opt/agent/jobs/)" ]; then
    echo "No jobs found in the /opt/agent/jobs/ directory." >&2
    exit 1
fi

for job in /opt/agent/jobs/*/Makefile; do
    make -C "$(dirname "$job")" build deploy
done