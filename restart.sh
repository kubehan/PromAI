#!/bin/bash
cd "$(dirname "$0")"
if [ -f .env.keys ]; then
  set -a
  . ./.env.keys
  set +a
fi
lsof -ti:8091 | xargs kill -9 2>/dev/null
sleep 1
go build -o promai . || exit 1
> nohup.out
nohup ./promai >> nohup.out 2>&1 &
echo "PromAI restarted on :8091 (pid $!)"
