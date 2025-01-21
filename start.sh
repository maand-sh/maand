#!/bin/bash
set -ueo pipefail

if [ -z "${1+x}" ]; then
  echo "Usage: maand <operation>"
  echo "Operations:"
  echo "  init                          Initialize the bucket"
  echo "  build                         Build bucket"
  echo "  deploy                        deploy bucket/jobs"
  echo "  run_command                   Run a command on the agents"
  echo "  job                           Run job control operations (start, stop and restart)"
  echo "  alloc_command                 Run job-related commands"
  echo "  cat                           Cat info from build action (agents, jobs, allocations, kv)"
  echo "  kv                            Key value store ops"
  echo "  health_check                  Run health checks"
  echo "  gc                            Garbage collect"
  exit 1
fi

export OPERATION=$1
shift

echo "StrictHostKeyChecking accept-new" >> /etc/ssh/ssh_config
rm -rf /bucket/logs/*
mkdir -p /opt/agents
python3 /maand/kv_manager.py

function run_python_script {
    script=$1
    shift
    python3 "/maand/$script" "$@"
}

function validate_ca_exists() {
    if [[ ! -f /bucket/secrets/ca.crt || ! -f /bucket/secrets/ca.key ]]; then
        echo "ca.key and/or ca.crt is not found."
        exit 1
    fi
}

case "$OPERATION" in
  "init")
    run_python_script "init.py"
    bash /maand/start.sh build
    ;;
  "info")
    validate_ca_exists
    run_python_script "cat.py" info
    ;;
  "build")
    validate_ca_exists
    run_python_script "build.py"
    ;;
  "deploy")
    validate_ca_exists
    run_python_script "deploy.py" "$@"
    ;;
  "job")
    validate_ca_exists
    run_python_script "job_control.py" "$@"
    ;;
  "health_check")
    validate_ca_exists
    run_python_script "health_check.py" "$@"
    ;;
  "alloc_command")
    export JOB=$1
    shift
    export COMMAND=$1
    shift
    validate_ca_exists
    run_python_script "alloc_command_executor.py" "$@"
    ;;
  "cat")
    validate_ca_exists
    run_python_script "cat.py" "$@"
    ;;
  "run_command")
    validate_ca_exists
    run_python_script "run_command.py" "$@"
    ;;
  "gc")
    validate_ca_exists
    run_python_script "gc.py"
    ;;
  *)
    echo "Unknown operation: $OPERATION"
    exit 1
    ;;
esac
