#!/bin/bash

RSYNC_PATH="rsync"
if [[ "$USE_SUDO" -eq 1 ]]; then
  RSYNC_PATH="sudo rsync"
fi

RSYNC_OPTIONS=" \
  -p -g -o \
  --ignore-times \
  --verbose \
  --force \
  --delete \
  --compress \
  --checksum \
  --recursive \
  --exclude=\"certs/reload.txt\" \
  --exclude=\"jobs/*/bin\" \
  --exclude=\"jobs/*/data\" \
  --exclude=\"jobs/*/logs\" \
  --filter='merge /tmp/${AGENT_IP}_rsync_rules.txt' \
"

rsync_command="rsync --rsync-path=\"$RSYNC_PATH\" $RSYNC_OPTIONS --rsh=\"ssh -i /bucket/$SSH_KEY\" $AGENT_DIR/ $SSH_USER@$AGENT_IP:/opt/agent/$BUCKET"
bash -c "$rsync_command" > /dev/null
