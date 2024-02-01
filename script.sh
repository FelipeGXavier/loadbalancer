#!/bin/bash

URL="http://localhost:8080/ping"
HEADER="X-Counter"

for counter in {1..100}; do
    echo "Making request $counter"
    curl -H "$HEADER: $counter" "$URL" &
done

# Wait for all background jobs to finish
wait