#!/bin/bash
set -ueo pipefail

for job in /opt/agent/jobs/*/Makefile; do
    make -C "$(dirname "$job")" build restart
done