#!/bin/bash

# install.sh - Installs manque-ai correctly

set -e

echo "🚀 Installing manque-ai..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first: https://go.dev/doc/install"
    exit 1
fi

# Install the binary
echo "📦 Installing from github.com/igcodinap/manque-ai@latest..."
go install github.com/igcodinap/manque-ai@latest

# Verify installation
if ! command -v manque-ai &> /dev/null; then
    echo "⚠️  manque-ai was installed but is not in your PATH."
    echo "💡 Add this to your shell profile (~/.zshrc or ~/.bashrc):"
    echo "   export PATH=\$PATH:\$(go env GOPATH)/bin"
    
    # Try to add it automatically if user agrees? No, let's keep it simple and safe.
    GOPATH=$(go env GOPATH)
    echo ""
    echo "You can run it directly with: $GOPATH/bin/manque-ai"
else
    echo "✅ manque-ai installed successfully!"
    manque-ai --help
fi
