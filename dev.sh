#!/bin/bash

# Backend development with auto-reload

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}🚀 Starting Backend with Auto-Reload${NC}"

# Install Air if not present
if ! command -v air &> /dev/null; then
    echo -e "${YELLOW}Installing Air...${NC}"
    go install github.com/air-verse/air@latest
    export PATH=$PATH:$(go env GOPATH)/bin
fi

# Clean up old processes
pkill -f "air" 2>/dev/null || true
pkill -f "tmp/main" 2>/dev/null || true

# Create tmp directory
mkdir -p tmp

echo -e "${GREEN}✅ Starting Air watcher...${NC}"
echo -e "${CYAN}   Watching: *.go, *.yaml, *.toml, *.env files${NC}"
echo -e "${CYAN}   Auto-restart on save${NC}"
echo -e "${YELLOW}   Press Ctrl+C to stop${NC}\n"

# Run Air
air -c .air.toml