#!/bin/bash

# Read input from stdin
INPUT=$(cat)

# CRITICAL: Prevent infinite loops - if we're already in a forced continuation, let it stop
STOP_HOOK_ACTIVE=$(echo "$INPUT" | jq -r '.stop_hook_active // false')
if [ "$STOP_HOOK_ACTIVE" = "true" ]; then
    exit 0
fi

# Run go build and capture output
BUILD_OUTPUT=$(go build ./... 2>&1)
BUILD_EXIT_CODE=$?

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    # Truncate output if too long
    TRUNCATED_OUTPUT=$(echo "$BUILD_OUTPUT" | tail -c 500)
    
    # Output JSON to block stopping - reason is fed back to Claude
    echo "{\"decision\": \"block\", \"reason\": \"go build ./... failed with errors:\\n${TRUNCATED_OUTPUT}\\n\\nPlease fix these compilation errors before completing.\"}"
    exit 0
fi

# Build succeeded, allow stopping
exit 0
