#!/bin/bash
rsync -rahzv --rsh="ssh -o StrictHostKeyChecking=no -o LogLevel=error -l $SSH_USER" /opt/agent/ $HOST:/opt/agent/
rsync -rahzv --delete --exclude 'bin' --exclude 'data' --exclude 'logs' --rsh="ssh -o StrictHostKeyChecking=no -o LogLevel=error -l $SSH_USER" /opt/agent/ $HOST:/opt/agent/