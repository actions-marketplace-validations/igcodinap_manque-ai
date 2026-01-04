#!/bin/bash

# Configuration test script - doesn't require real API calls
echo "🧪 Testing manque-ai configuration..."

# Test environment variables
echo "📋 Checking environment..."
echo "LLM_PROVIDER: ${LLM_PROVIDER:-openrouter}"
echo "LLM_MODEL: ${LLM_MODEL:-mistralai/mistral-7b-instruct:free}"
echo "GH_TOKEN: ${GH_TOKEN:+SET} ${GH_TOKEN:-NOT_SET}"
echo "LLM_API_KEY: ${LLM_API_KEY:+SET} ${LLM_API_KEY:-NOT_SET}"
echo ""

# Test build
echo "🔨 Testing build..."
go build -o manque-ai .
if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed"
    exit 1
fi

# Test help command
echo ""
echo "📖 Testing help command..."
./manque-ai --help

echo ""
echo "🎯 To run a real test, set your credentials:"
echo "export GH_TOKEN=your_token"
echo "export LLM_API_KEY=sk-or-v1-your-key" 
echo "export LLM_MODEL=mistralai/mistral-7b-instruct:free"
echo ""
echo "Then run: ./test-local.sh"