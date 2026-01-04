#!/bin/bash

# Local testing script for manque-ai with OpenRouter
echo "🧪 Testing manque-ai with OpenRouter..."

# Check if required environment variables are set
if [ -z "$GH_TOKEN" ]; then
    echo "❌ GH_TOKEN not set. Please run: export GH_TOKEN=your_token"
    exit 1
fi

if [ -z "$LLM_API_KEY" ]; then
    echo "❌ LLM_API_KEY not set. Please run: export LLM_API_KEY=sk-or-v1-your-key"
    exit 1
fi

# Set default values for testing
export LLM_PROVIDER=${LLM_PROVIDER:-openrouter}
export LLM_MODEL=${LLM_MODEL:-mistralai/mistral-7b-instruct:free}

echo "📋 Configuration:"
echo "  Provider: $LLM_PROVIDER"
echo "  Model: $LLM_MODEL"
echo "  API Key: ${LLM_API_KEY:0:20}..."
echo ""

# Build the project
echo "🔨 Building manque-ai..."
go build -o manque-ai .

if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi

echo "✅ Build successful!"
echo ""

# Test with a small, recent PR
echo "🚀 Testing with a sample PR..."
echo "Using: https://github.com/golang/go/pull/70456 (small Go PR for testing)"
echo ""

./manque-ai --url https://github.com/golang/go/pull/70456

echo ""
echo "🎉 Test completed!"