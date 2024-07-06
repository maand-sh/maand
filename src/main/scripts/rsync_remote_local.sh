#!/bin/bash
mkdir -p /opt/agent
rsync -rahzv --rsh="ssh -o StrictHostKeyChecking=no -o LogLevel=error -l $SSH_USER" --include='*.txt' --exclude='*' "$NODE_IP":/opt/agent/ /opt/agent/