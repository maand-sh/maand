#!/bin/bash
set -e

# Define constants
RSYNC="rsync"
DEFAULT_TIMEOUT=30
RSYNC_EXCLUDES=(
  "--exclude=jobs/*/bin"
  "--exclude=jobs/*/data"
  "--exclude=jobs/*/logs"
)
RSYNC_OPTIONS=(
  "--timeout=$DEFAULT_TIMEOUT"
  "--inplace"
  "--whole-file"
  "--checksum"
  "--recursive"
  "--force"
  "--delete-after"
  "--delete"
  "--group"
  "--owner"
  "--executability"
  "--compress"
)

# Check required variables
if [[ -z "$SSH_KEY" || -z "$AGENT_DIR" || -z "$SSH_USER" || -z "$AGENT_IP" || -z "$BUCKET" ]]; then
  echo "Error: Missing required environment variables (SSH_KEY, AGENT_DIR, SSH_USER, AGENT_IP, or BUCKET)." >&2
  exit 1
fi

# Enable sudo for rsync if specified
if [[ "${USE_SUDO:-0}" -eq 1 ]]; then
  RSYNC="sudo rsync"
fi

# Validate input arguments
if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <rule_file>" >&2
  exit 1
fi
RULE_FILE=$1

# Construct the rsync command
RSYNC_CMD=(
  $RSYNC -v
  --rsync-path="$RSYNC"
  "${RSYNC_OPTIONS[@]}"
  "${RSYNC_EXCLUDES[@]}"
  --filter="merge /tmp/${RULE_FILE}.txt"
  --rsh="ssh -o BatchMode=true -o ConnectTimeout=10 -i /bucket/$SSH_KEY"
  "$AGENT_DIR/"
  "$SSH_USER@$AGENT_IP:/opt/agent/$BUCKET"
)

# Execute the rsync command
"${RSYNC_CMD[@]}" > /dev/null
