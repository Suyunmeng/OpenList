#!/bin/bash

# Driver Manager Startup Script

echo "Starting OpenList Driver Manager..."

# Set default values
OPENLIST_HOST=${OPENLIST_HOST:-"localhost"}
OPENLIST_PORT=${OPENLIST_PORT:-5245}
MANAGER_ID=${DRIVER_MANAGER_ID:-""}
RECONNECT_INTERVAL=${RECONNECT_INTERVAL:-"5s"}

# Build the driver manager
echo "Building driver manager..."
go build -o driver-manager main.go

if [ $? -ne 0 ]; then
    echo "Failed to build driver manager"
    exit 1
fi

# Start the driver manager
echo "Starting driver manager connecting to OpenList at ${OPENLIST_HOST}:${OPENLIST_PORT}"
./driver-manager \
    -openlist-host="${OPENLIST_HOST}" \
    -openlist-port="${OPENLIST_PORT}" \
    -manager-id="${MANAGER_ID}" \
    -reconnect-interval="${RECONNECT_INTERVAL}"