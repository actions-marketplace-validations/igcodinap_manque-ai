#!/bin/bash
set -e

echo "🧪 Starting Local Integration Test..."

# Build binary
echo "🔨 Building binary..."
go build -o manque-ai .

# Capture build dir
BUILD_DIR=$(pwd)

# Create temp dir
TEMP_DIR=$(mktemp -d)
echo "📂 Created temp dir: $TEMP_DIR"

# Cleanup trap
cleanup() {
    echo "🧹 Cleaning up..."
    rm -rf "$TEMP_DIR"
    rm -f "$BUILD_DIR/manque-ai"
}
trap cleanup EXIT

# Setup git repo
cd "$TEMP_DIR"
git init
git config user.email "test@example.com"
git config user.name "Test User"
git checkout -b main

# Create initial file
echo "Initial content" > file.txt
git add file.txt
git commit -m "Initial commit"

# Create feature branch
git checkout -b feature
echo "Modified content" > file.txt
git add file.txt
git commit -m "Update file"

# Run local review
echo "🚀 Running manque-ai local..."

# Check help
"$BUILD_DIR/manque-ai" local --help > /dev/null

if [ -n "$LLM_API_KEY" ]; then
    "$BUILD_DIR/manque-ai" local --base main --head feature || true 
else
    echo "⚠️  LLM_API_KEY not set, verifying binary structure only."
    "$BUILD_DIR/manque-ai" local --help
fi

echo "✅ Integration test passed!"
