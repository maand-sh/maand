#!/bin/bash
set -ueo pipefail

if [ -z "${1+x}" ]; then
  echo "Usage: maand <operation>"
  echo "Operations:"
  echo "  init                          Initialize the bucket"
  echo "  build                         Build bucket"
  echo "  deploy                        deploy jobs"
  echo "  uptime                        Check connectivity or uptime"
  echo "  run_command                   Run a command on the agents"
  echo "  job                           Run job control operations (start, stop and restart)"
  echo "  alloc_command                 Run job-related commands"
  echo "  cat                           Cat info from build action (agents, jobs, allocations, kv)"
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

case "$OPERATION" in
  "init")
    run_python_script "init.py"
    bash /maand/start.sh build
    ;;
  "info")
    run_python_script "cat.py" info
    ;;
  "build")
    run_python_script "build.py"
    ;;
  "deploy")
    run_python_script "deploy.py" "$@"
    ;;
  "job")
    run_python_script "job_control.py" "$@"
    ;;
  "health_check")
    run_python_script "health_check.py" "$@"
    ;;
  "alloc_command")
    export JOB=$1
    shift
    export COMMAND=$1
    shift
    run_python_script "alloc_command_executor.py" "$@"
    ;;
  "cat")
    run_python_script "cat.py" "$@"
    ;;
  "uptime")
    run_python_script "uptime.py" "$@"
    ;;
  "run_command")
    run_python_script "run_command.py" "$@"
    ;;
  "gc")
    run_python_script "gc.py"
    ;;
  *)
    echo "Unknown operation: $OPERATION"
    exit 1
    ;;
esac
