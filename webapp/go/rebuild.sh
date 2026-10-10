#!/bin/bash
set -euo pipefail
cd /home/isucon/webapp/go
docker run --rm --network host -v "$PWD":/app -w /app golang:1.18.4-bullseye go build -o isuports ./cmd/isuports/main.go
sudo chown isucon:isucon isuports
sudo systemctl restart isuports
