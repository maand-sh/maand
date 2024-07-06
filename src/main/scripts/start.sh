#!/bin/bash
set -ueo pipefail

# shellcheck disable=SC2046
eval $(ssh-agent -s) > /dev/null

touch /workspace/variables.env
source /workspace/variables.env

export OPERATION=$1
export CLUSTER_ID=${CLUSTER_ID:-"undefined"}
export NETWORK_INTERFACE_NAME=${NETWORK_INTERFACE_NAME:-"eth0"}
export SSH_USER=${SSH_USER:-""}
export SSH_KEY=${SSH_KEY:-""}
export IMAGE_NAME=$(docker inspect --format='{{.Config.Image}}' "$HOSTNAME")
export NODE_OPS=${NODE_OPS:-"0"}
export MAX_CONCURRENCY=${MAX_CONCURRENCY:-"4"}
export WORKSPACE=${WORKSPACE:-""}

mkdir -p /opt/agents

if [[ -z "$OPERATION" || -z "$SSH_USER" || -z "$SSH_KEY" || -z "$WORKSPACE" ]]; then
  echo "missing arguments (OPERATION, SSH_USER, SSH_KEY, WORKSPACE)";
  exit 1
fi

ssh-add /workspace/"${SSH_KEY}" 2> /dev/null
echo "StrictHostKeyChecking accept-new" >> /etc/ssh/ssh_config

if [ "$NODE_OPS" == "1" ]; then
  python3 /scripts/"node_ops_$OPERATION".py
  exit 0
fi

if [ "$OPERATION" == "run_command" ]; then
  roles=${2:-""}
  max_concurrency=${MAX_CONCURRENCY:-0}
  ignore_error=${IGNORE_ERROR:-0}
  touch /workspace/command.sh && python3 /scripts/system_manager.py --roles "$roles" --concurrency "$max_concurrency" --ignore_error "$ignore_error" --operation run_command
elif [ "$OPERATION" == "bootstrap" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation linux_setup
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation update
elif [ "$OPERATION" == "linux_patching" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation linux_patching
elif [ "$OPERATION" == "sync" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation update
elif [ "$OPERATION" == "deploy_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation deploy_jobs
elif [ "$OPERATION" == "restart_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation restart_jobs
elif [ "$OPERATION" == "stop_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation stop_jobs
fi