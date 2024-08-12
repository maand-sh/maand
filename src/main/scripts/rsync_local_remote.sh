#!/bin/bash
rsync -rahz --quiet --rsh="ssh -o StrictHostKeyChecking=no -o LogLevel=error -l $SSH_USER" /opt/agent/ "$AGENT_IP":/opt/agent/ > /dev/null
rsync -rahz --quiet --rsh="ssh -o StrictHostKeyChecking=no -o LogLevel=error -l $SSH_USER" --delete --exclude 'bin' --exclude 'data' --exclude 'logs' /opt/agent/ "$AGENT_IP":/opt/agent/ > /dev/null