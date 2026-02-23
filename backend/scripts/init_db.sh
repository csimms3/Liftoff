#!/bin/bash

# Database initialization script for Liftoff

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Initializing Liftoff Database...${NC}"

# Check if PostgreSQL is running
if ! pg_isready -q; then
    echo -e "${RED}PostgreSQL is not running. Please start PostgreSQL first.${NC}"
    exit 1
fi

# Create database if it doesn't exist
echo -e "${YELLOW}Creating database 'liftoff' if it doesn't exist...${NC}"
createdb -h localhost -U postgres liftoff 2>/dev/null || echo "Database 'liftoff' already exists"

# Run migrations
echo -e "${YELLOW}Running database migrations...${NC}"
psql -h localhost -U postgres -d liftoff -f ../migrations/001_initial_schema.sql

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Database initialized successfully!${NC}"
    echo -e "${GREEN}Database: liftoff${NC}"
    echo -e "${GREEN}Connection: postgres://postgres:password@localhost:5432/liftoff${NC}"
else
    echo -e "${RED}Failed to initialize database${NC}"
    exit 1
fi
