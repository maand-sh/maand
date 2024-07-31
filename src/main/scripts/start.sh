#!/bin/bash
set -ueo pipefail

# shellcheck disable=SC2046
eval $(ssh-agent -s) > /dev/null

touch /workspace/variables.env
source /workspace/variables.env

export OPERATION=$1
export CLUSTER_ID=${CLUSTER_ID:-"undefined"}
export NETWORK_INTERFACE_NAME=${NETWORK_INTERFACE_NAME:-"eth0"}
export UPDATE_CERTS=${UPDATE_CERTS:-0}
export SSH_USER=${SSH_USER:-""}
export SSH_KEY=${SSH_KEY:-""}
export IMAGE_NAME=$(docker inspect --format='{{.Config.Image}}' "$HOSTNAME")
export NODE_OPS=${NODE_OPS:-"0"}
export MAX_CONCURRENCY=${MAX_CONCURRENCY:-"4"}
export WORKSPACE=${WORKSPACE:-""}

mkdir -p /opt/agents

if [[ -z "$OPERATION" || -z "$SSH_USER" || -z "$SSH_KEY" || -z "$WORKSPACE" || -z "$CLUSTER_ID" ]]; then
  echo "missing arguments (OPERATION, SSH_USER, SSH_KEY, WORKSPACE, CLUSTER_ID)";
  exit 1
fi

ssh-add /workspace/"${SSH_KEY}" 2> /dev/null
echo "StrictHostKeyChecking accept-new" >> /etc/ssh/ssh_config

if [ "$NODE_OPS" == "1" ]; then
  python3 /scripts/"node_ops_$OPERATION".py
  exit 0
fi

if [ ! -f /workspace/ca.key ]; then
  openssl genrsa -out /workspace/ca.key 4096
  openssl req -new -x509 -sha256 -days 365 -subj /CN="$CLUSTER_ID" -key /workspace/ca.key -out /workspace/ca.crt
fi

if [ "$OPERATION" == "run_command" ]; then
  roles=${2:-""}
  max_concurrency=${MAX_CONCURRENCY:-0}
  ignore_error=${IGNORE_ERROR:-0}
  touch /workspace/command.sh && python3 /scripts/system_manager.py --roles "$roles" --concurrency "$max_concurrency" --ignore_error "$ignore_error" --operation run_command
elif [ "$OPERATION" == "sync" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation update
elif [ "$OPERATION" == "deploy_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation deploy_jobs
elif [ "$OPERATION" == "stop_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation stop_jobs
elif [ "$OPERATION" == "restart_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation restart_jobs
elif [ "$OPERATION" == "force_deploy_jobs" ]; then
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation force_deploy_jobs
elif [ "$OPERATION" == "rolling_upgrade" ]; then
  python3 /scripts/system_manager.py --concurrency "1" --operation rolling_upgrade
elif [ "$OPERATION" == "health_check" ]; then
  export MODULE="health_check"
  python3 /scripts/system_manager.py --concurrency "$MAX_CONCURRENCY" --operation run_module
fi