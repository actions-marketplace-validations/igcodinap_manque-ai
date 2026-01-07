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
GOPATH=$(go env GOPATH)
GOBIN="$GOPATH/bin"

if ! command -v manque-ai &> /dev/null; then
    echo "⚠️  manque-ai was installed but is not in your PATH."

    # Detect shell profile
    SHELL_NAME=$(basename "$SHELL")
    case "$SHELL_NAME" in
        zsh)  PROFILE="$HOME/.zshrc" ;;
        bash)
            if [[ -f "$HOME/.bash_profile" ]]; then
                PROFILE="$HOME/.bash_profile"
            else
                PROFILE="$HOME/.bashrc"
            fi
            ;;
        *)    PROFILE="$HOME/.profile" ;;
    esac

    # Check if GOPATH/bin is already in the profile
    EXPORT_LINE='export PATH="$PATH:$(go env GOPATH)/bin"'
    if grep -q 'go env GOPATH.*bin' "$PROFILE" 2>/dev/null || grep -q "$GOBIN" "$PROFILE" 2>/dev/null; then
        echo "💡 GOPATH/bin is already in $PROFILE but not active in this session."
        echo "   Run: source $PROFILE"
    else
        echo "📝 Adding GOPATH/bin to $PROFILE..."
        echo "" >> "$PROFILE"
        echo "# Added by manque-ai installer" >> "$PROFILE"
        echo "$EXPORT_LINE" >> "$PROFILE"
        echo "✅ Added to $PROFILE"
        echo ""
        echo "🔄 To use manque-ai now, run:"
        echo "   source $PROFILE"
    fi
else
    echo "✅ manque-ai installed successfully!"
    manque-ai --help
fi
