# AI Code Reviewer

A robust Golang binary that reviews Pull Requests using LLMs (OpenAI, Anthropic, Google). Provides instant PR summaries, line-by-line code review comments, and intelligent title generation.

## Features

- **Multi-Provider LLM Support**: OpenAI, Anthropic Claude, Google Gemini
- **Dual Mode Operation**: GitHub Action or standalone CLI tool
- **Intelligent PR Analysis**: Generates summaries, reviews, and suggestions
- **Structured Output**: JSON-based responses with consistent formatting
- **Security Focus**: Prioritizes security vulnerabilities and code quality
- **Style Guide Integration**: Custom project-specific rules support
- **GitHub Integration**: Automatic PR updates and inline comments

## Installation

### As a GitHub Action

#### Step 1: Add API Key to Repository Secrets

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Add your LLM API key:
   - **Name**: `OPENAI_API_KEY` (or `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`, `OPENROUTER_API_KEY`)
   - **Value**: Your actual API key (e.g., `sk-...`, `sk-or-v1-...`)

#### Step 2: Create the Workflow File

Create `.github/workflows/ai-review.yml` in your repository:

```yaml
name: AI Code Review
on:
  pull_request:
    types: [opened, synchronize, reopened]
  pull_request_review_comment:
    types: [created]

jobs:
  ai-review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      issues: write
    steps:
      - name: AI Code Review
        uses: docker://ghcr.io/manque-ai/manque-ai:latest
        env:
          GH_TOKEN: ${{ secrets.GH_TOKEN }}
          LLM_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          LLM_PROVIDER: "openai"
          LLM_MODEL: "gpt-4o"
          GITHUB_EVENT_PATH: ${{ github.event_path }}
```

#### Step 3: Customize Configuration (Optional)

You can customize the behavior by adding more environment variables:

```yaml
# Using Anthropic Claude
      - name: AI Code Review (Anthropic)
        uses: docker://ghcr.io/manque-ai/manque-ai:latest
        env:
          GH_TOKEN: ${{ secrets.GH_TOKEN }}
          LLM_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          LLM_PROVIDER: "anthropic"
          LLM_MODEL: "claude-3-5-sonnet-20241022"
          UPDATE_PR_TITLE: "true"
          UPDATE_PR_BODY: "true"
          STYLE_GUIDE_RULES: |
            - Use TypeScript strict mode
            - Prefer composition over inheritance
            - All functions must have JSDoc comments
            - Use meaningful variable names
          GITHUB_EVENT_PATH: ${{ github.event_path }}

# Using OpenRouter (Multiple Models Available)
      - name: AI Code Review (OpenRouter)
        uses: docker://ghcr.io/manque-ai/manque-ai:latest
        env:
          GH_TOKEN: ${{ secrets.GH_TOKEN }}
          LLM_API_KEY: ${{ secrets.OPENROUTER_API_KEY }}
          LLM_PROVIDER: "openrouter"
          LLM_MODEL: "anthropic/claude-3.5-sonnet"  # or any OpenRouter model
          UPDATE_PR_TITLE: "true"
          UPDATE_PR_BODY: "true"
          GITHUB_EVENT_PATH: ${{ github.event_path }}
```

#### When the Action Runs

The AI reviewer will automatically run when:
- ✅ **New PR is opened** (`opened`)
- ✅ **New commits are pushed to PR** (`synchronize`) 
- ✅ **PR is reopened** (`reopened`)
- ✅ **Review comment is added** (`pull_request_review_comment`)

#### What the Action Does

1. **Analyzes the entire PR diff**
2. **Generates an AI summary** with file-by-file breakdown
3. **Creates inline code review comments** for issues found
4. **Posts a walkthrough comment** with overall assessment
5. **Optionally updates PR title and description** with AI insights

#### Alternative: Use Local Dockerfile

If you prefer to build from source, you can reference the repository directly:

```yaml
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          repository: manque-ai/manque-ai
          path: ai-reviewer
      
      - name: Run AI Review
        run: |
          cd ai-reviewer
          docker build -t manque-ai .
          docker run --rm \
            -e GH_TOKEN="${{ secrets.GH_TOKEN }}" \
            -e LLM_API_KEY="${{ secrets.OPENAI_API_KEY }}" \
            -e LLM_PROVIDER="openai" \
            -e LLM_MODEL="gpt-4o" \
            -e GITHUB_EVENT_PATH="${{ github.event_path }}" \
            manque-ai
```

### As a CLI Tool

```bash
# Download binary
go install github.com/manque-ai@latest

# Or build from source
git clone https://github.com/manque-ai/ai-reviewer
cd ai-reviewer
go build -o ai-reviewer
```

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `GH_TOKEN` | GitHub API token | ✅ | - |
| `LLM_API_KEY` | LLM provider API key | ✅ | - |
| `LLM_PROVIDER` | LLM provider (openai, anthropic, google, openrouter) | ❌ | `openai` |
| `LLM_MODEL` | Model name | ❌ | `gpt-4o` |
| `LLM_BASE_URL` | Custom API endpoint | ❌ | - |
| `GITHUB_API_URL` | GitHub API URL | ❌ | `https://api.github.com` |
| `STYLE_GUIDE_RULES` | Custom style guide | ❌ | - |
| `UPDATE_PR_TITLE` | Update PR title | ❌ | `true` |
| `UPDATE_PR_BODY` | Update PR body | ❌ | `true` |

### GitHub Action Inputs

All environment variables are also available as action inputs in lowercase:

```yaml
- uses: manque-ai/ai-reviewer@v1
  with:
    github_token: ${{ secrets.GH_TOKEN }}
    llm_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
    llm_provider: "anthropic"
    llm_model: "claude-3-5-sonnet-20241022"
    style_guide_rules: |
      - Use TypeScript strict mode
      - Prefer composition over inheritance
      - All functions must have JSDoc comments
```

## Usage

### CLI Mode

```bash
# Review specific PR by number
ai-reviewer --repo owner/repo --pr 123

# Review PR by URL
ai-reviewer --url https://github.com/owner/repo/pull/123

# Set environment variables for OpenAI
export GH_TOKEN=your_token
export LLM_API_KEY=your_api_key
export LLM_PROVIDER=openai
ai-reviewer --repo owner/repo --pr 123

# Or use OpenRouter with any model
export GH_TOKEN=your_token
export LLM_API_KEY=sk-or-v1-your-key
export LLM_PROVIDER=openrouter
export LLM_MODEL=anthropic/claude-3.5-sonnet
ai-reviewer --repo owner/repo --pr 123
```

### GitHub Action Mode

The action automatically runs when PRs are opened or updated, reading context from `GITHUB_EVENT_PATH`.

## LLM Provider Setup

### OpenAI
```bash
export LLM_PROVIDER=openai
export LLM_API_KEY=sk-...
export LLM_MODEL=gpt-4o  # or gpt-4, gpt-3.5-turbo
```

### Anthropic Claude
```bash
export LLM_PROVIDER=anthropic
export LLM_API_KEY=sk-ant-...
export LLM_MODEL=claude-3-5-sonnet-20241022
```

### Google Gemini
```bash
export LLM_PROVIDER=google
export LLM_API_KEY=your_google_api_key
export LLM_MODEL=gemini-pro
```

### OpenRouter (Multiple Models)
```bash
export LLM_PROVIDER=openrouter
export LLM_API_KEY=sk-or-v1-...
export LLM_MODEL=anthropic/claude-3.5-sonnet  # or any supported model
```

Popular OpenRouter models:
- `anthropic/claude-3.5-sonnet` - Excellent for code review
- `openai/gpt-4o` - Good all-around performance  
- `google/gemini-pro-1.5` - Strong reasoning capabilities
- `meta-llama/llama-3.1-70b-instruct` - Open source alternative
- `qwen/qwen-2.5-72b-instruct` - High quality, cost-effective
- `deepseek/deepseek-coder-v2` - Specialized for code analysis

See [OpenRouter models](https://openrouter.ai/models) for the complete list.

### Custom OpenAI-Compatible Provider
```bash
export LLM_PROVIDER=openai
export LLM_BASE_URL=https://api.your-provider.com/v1
export LLM_API_KEY=your_key
```

## Output Format

### PR Summary
```json
{
  "title": "Add user authentication system",
  "description": "Implements JWT-based authentication with login/logout functionality",
  "type": ["FEATURE", "SECURITY"],
  "files": [
    {
      "filename": "src/auth.ts",
      "summary": "Core authentication logic with JWT token handling",
      "title": "Authentication implementation"
    }
  ]
}
```

### Code Review
```json
{
  "review": {
    "estimated_effort_to_review": 3,
    "score": 85,
    "has_relevant_tests": true,
    "security_concerns": "No significant security issues detected"
  },
  "comments": [
    {
      "file": "src/auth.ts",
      "start_line": 15,
      "end_line": 18,
      "highlighted_code": "const token = jwt.sign(payload, secret);",
      "header": "🟡 Missing error handling",
      "content": "JWT signing can throw errors...",
      "label": "bug",
      "critical": false
    }
  ]
}
```

## Architecture

```
├── cmd/                    # CLI interface
├── internal/              # Internal packages
│   ├── config.go         # Configuration management
│   └── logger.go         # Logging utilities
├── pkg/
│   ├── ai/               # LLM client abstraction
│   │   ├── client.go     # Base client
│   │   ├── openai.go     # OpenAI implementation
│   │   ├── anthropic.go  # Anthropic implementation
│   │   ├── google.go     # Google implementation
│   │   ├── prompts.go    # System prompts
│   │   └── types.go      # Type definitions
│   ├── diff/             # Git diff parsing
│   │   └── parser.go     # Diff parser with LLM formatting
│   └── github/           # GitHub API client
│       └── client.go     # GitHub operations
├── main.go               # Application entry point
├── Dockerfile            # Container for GitHub Actions
└── action.yml            # GitHub Action definition
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
