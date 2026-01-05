![Manque AI Banner](public/manque-banner.png)

<div align="center">

# AI Code Reviewer

[![Go Report Card](https://goreportcard.com/badge/github.com/manque-ai/manque-ai)](https://goreportcard.com/report/github.com/manque-ai/manque-ai)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Image Version](https://img.shields.io/docker/v/igcodinap/manque-ai?sort=semver)](https://hub.docker.com/r/igcodinap/manque-ai)

**Your Intelligent AI Pair Programmer for GitHub Pull Requests.**
*Instant summaries, deep code analysis, and actionable security insights.*

</div>

---

## ✨ Features

- **🚀 Dual Mode Operation**: Run as a GitHub Action or a local CLI tool.
- **🤖 Multi-Provider LLM Support**: First-class support for OpenAI, Anthropic Claude, Google Gemini, and OpenRouter.
- **🧠 Intelligent Analysis**: Generates executive summaries, walkthroughs, and line-by-line review comments.
- **🔒 Security First**: dedicated analysis for hardcoded secrets and potential vulnerabilities.
- **💻 Local Pre-PR Checks**: Review your code locally before you even push.
- **🎨 Custom Styling**: Enforce your team's unique style guide and best practices.

---

## 💻 Local Development (Pre-PR Check)

Review your changes locally without pushing to GitHub. This is perfect for catching issues early!

### 1. Installation
```bash
go install github.com/manque-ai/manque-ai@latest
# or build from source
git clone https://github.com/manque-ai/manque-ai
cd manque-ai && go build -o ai-reviewer .
```

### 2. Setup (One-time)
You can set your LLM credentials as environment variables or using a **`.env` file** in the project root. **Note: `GH_TOKEN` is OPTIONAL for local runs!**

#### Option A: Using a .env file (Recommended)
Copy the example file and fill in your keys:
```bash
cp .env.example .env
# Edit .env and add your LLM_API_KEY
```

#### Option B: Exporting variables
```bash
# OpenAI
export LLM_PROVIDER=openai
export LLM_API_KEY=sk-...
```

### 3. Run Review
```bash
# Review changes in your current branch vs main
ai-reviewer local

# Compare specific branches
ai-reviewer local --base develop --head feature-login

# Debug mode (see exact API calls and diff sizes)
ai-reviewer local --debug
```

---

## 🚀 GitHub Action Usage

Integrate directly into your CI/CD pipeline to review every Pull Request automatically.

```yaml
name: AI Code Review
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - name: AI Reviewer
        uses: docker://ghcr.io/igcodinap/manque-ai:latest
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          LLM_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          LLM_PROVIDER: "openai"
          LLM_MODEL: "gpt-4o"
```

### Configuration Options

| Variable | Description | Required (Action) | Required (Local) | Default |
|----------|-------------|-------------------|------------------|---------|
| `GH_TOKEN` | GitHub API Token | ✅ | ❌ | - |
| `LLM_API_KEY` | LLM Provider Key | ✅ | ✅ | - |
| `LLM_PROVIDER` | `openai`, `anthropic`, `google`, `openrouter` | ❌ | ❌ | `openai` |
| `LLM_MODEL` | Specific model ID | ❌ | ❌ | `gpt-4o` |
| `STYLE_GUIDE_RULES`| Custom instructions for the AI | ❌ | ❌ | - |
| `UPDATE_PR_TITLE`| Auto-update PR title | ❌ | N/A | `true` |
| `UPDATE_PR_BODY` | Auto-update PR description | ❌ | N/A | `true` |

---

## 🛠️ Advanced CLI Usage

The CLI can also be used to review remote PRs or check GitHub Actions context.

```bash
# Review a specific remote PR
ai-reviewer --repo owner/repo --pr 123

# Review by URL
ai-reviewer --url https://github.com/owner/repo/pull/123
```

---

## 🧠 Architecture

The project is built with modularity in mind, separating the "brain" from the interface.

```
├── cmd/               # CLI Commands
│   ├── root.go        # GitHub Action / Remote Review
│   └── local.go       # Local Pre-PR Review
├── pkg/
│   ├── review/        # Core Review Engine (Shared Logic)
│   ├── ai/            # LLM Client Adapters
│   ├── diff/          # Git Diff Parser
│   └── github/        # GitHub API Client
└── internal/          # Config & Logging
```

## 🤝 Contributing

We love contributions! Please fork the repository and submit a Pull Request.

## 📄 License

MIT Licensed. default_api.
