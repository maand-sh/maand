#!/bin/bash
set -e

RSYNC_PATH="rsync"
if [[ "$USE_SUDO" -eq 1 ]]; then
  RSYNC_PATH="sudo rsync"
fi

RULE_FILE=$1

RSYNC_OPTIONS=" \
  --timeout 30    \
  --inplace       \
  --whole-file    \
  --checksum      \
  --recursive     \
  --force         \
  --delete-after  \
  --delete        \
  --exclude=\"jobs/*/bin\"  \
  --exclude=\"jobs/*/data\" \
  --exclude=\"jobs/*/logs\" \
  --filter='merge /tmp/$RULE_FILE.txt' \
  --group         \
  --owner         \
  --executability \
  --compress      \
"

echo
echo "updating file for $AGENT_IP"

rsync_command="rsync -v --rsync-path=\"$RSYNC_PATH\" $RSYNC_OPTIONS --rsh=\"ssh -o BatchMode=true -o ConnectTimeout=10 -i /bucket/$SSH_KEY\" $AGENT_DIR/ $SSH_USER@$AGENT_IP:/opt/agent/$BUCKET"
bash -c "$rsync_command"> /dev/null
