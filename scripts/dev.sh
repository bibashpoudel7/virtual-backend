#!/bin/bash

# Development script with auto-reload for backend

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Starting Development Environment${NC}"

# Function to cleanup on exit
cleanup() {
    echo -e "\n${YELLOW}🛑 Shutting down...${NC}"
    pkill -f "air" 2>/dev/null || true
    pkill -f "tmp/main" 2>/dev/null || true
    exit 0
}

# Trap exit signals
trap cleanup INT TERM

# Check if Air is installed
if ! command -v air &> /dev/null; then
    echo -e "${YELLOW}📦 Air not found. Installing...${NC}"
    go install github.com/cosmtrek/air@latest
    
    # Add Go bin to PATH if not already there
    export PATH=$PATH:$(go env GOPATH)/bin
fi

# Check if database is running
if ! nc -z localhost 5432 2>/dev/null; then
    echo -e "${YELLOW}⚠️  PostgreSQL not detected on port 5432${NC}"
    echo -e "${YELLOW}   Make sure your database is running${NC}"
fi

# Create tmp directory if it doesn't exist
mkdir -p tmp

# Clear old logs
rm -f tmp/*.log

echo -e "${GREEN}✅ Environment ready${NC}"
echo -e "${GREEN}👀 Watching for file changes...${NC}"
echo -e "${YELLOW}   Press Ctrl+C to stop${NC}\n"

# Start Air with custom configuration
air -c .air.toml

# Keep script running
wait