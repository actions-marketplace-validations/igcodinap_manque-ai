![Manque AI Banner](public/manque-banner.png)

<div align="center">

# AI Code Reviewer

[![Go Report Card](https://goreportcard.com/badge/github.com/igcodinap/manque-ai)](https://goreportcard.com/report/github.com/igcodinap/manque-ai)
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
**Quick Install (Recommended)**
```bash
curl -sSL https://raw.githubusercontent.com/igcodinap/manque-ai/main/install.sh | bash
```

**Manual Install**
```bash
go install github.com/igcodinap/manque-ai@latest
```

### 2. Configuration

**Interactive Setup (Recommended)**
```bash
manque-ai config init
```

This wizard will guide you through setting up your LLM provider and API key. Config is saved to `~/.manque-ai/config.yaml`.

**Other config commands:**
```bash
manque-ai config show              # View current configuration
manque-ai config set provider openai   # Set provider
manque-ai config set api_key sk-xxx    # Set API key
manque-ai config set model gpt-4o      # Set model
manque-ai config clear             # Remove configuration
```

**Alternative: Environment Variables**

You can also use environment variables (they override config file):
```bash
export LLM_PROVIDER=openrouter
export LLM_API_KEY=sk-or-...
export LLM_MODEL=mistralai/mistral-7b-instruct:free
```

<details>
<summary>Provider Examples</summary>

**OpenRouter (Default - Free tier available)**
```bash
export LLM_PROVIDER=openrouter
export LLM_API_KEY=sk-or-...
# Default model: mistralai/mistral-7b-instruct:free
```

**OpenAI**
```bash
export LLM_PROVIDER=openai
export LLM_API_KEY=sk-...
export LLM_MODEL=gpt-4o
```

**Anthropic**
```bash
export LLM_PROVIDER=anthropic
export LLM_API_KEY=sk-ant-...
export LLM_MODEL=claude-sonnet-4-20250514
```

**Local Ollama**
```bash
export LLM_PROVIDER=openai
export LLM_BASE_URL=http://localhost:11434/v1
export LLM_API_KEY=ollama
export LLM_MODEL=llama3
```

</details>

### 3. Run Review
```bash
# Review changes in your current branch vs main
manque-ai local

# Compare specific branches
manque-ai local --base develop --head feature-login

# Debug mode (see exact API calls and diff sizes)
manque-ai local --debug
```

### 4. Update
```bash
manque-ai update
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
      - name: Manque AI
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
| `LLM_PROVIDER` | `openai`, `anthropic`, `google`, `openrouter` | ❌ | ❌ | `openrouter` |
| `LLM_MODEL` | Specific model ID | ❌ | ❌ | `mistralai/mistral-7b-instruct:free` |
| `STYLE_GUIDE_RULES`| Custom instructions for the AI | ❌ | ❌ | - |
| `UPDATE_PR_TITLE`| Auto-update PR title | ❌ | N/A | `true` |
| `UPDATE_PR_BODY` | Auto-update PR description | ❌ | N/A | `true` |

---

## 🛠️ Advanced CLI Usage

The CLI can also be used to review remote PRs or check GitHub Actions context.

```bash
# Review a specific remote PR
manque-ai --repo owner/repo --pr 123

# Review by URL
manque-ai --url https://github.com/owner/repo/pull/123
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

We love contributions! Please fork the repository and submit a Pull Request. Or open a bug if you notice any.

Made with ❤️ from [Concon, Chile](https://www.google.com/search?sca_esv=760e5d65b7c9552a&sxsrf=AE3TifP-z7Necb_Ujj8y174gmieGKqR2cQ:1767800457086&udm=2&fbs=AIIjpHx4nJjfGojPVHhEACUHPiMQht6_BFq6vBIoFFRK7qchKHDX9TtpZ992kyQpCWcw0Wi-YYs4nsdnC8KGzOR7VwQCyjKvnt6zuwTJi0DnLtY3Dcu8lgFlRf1np4JvrceUHuyA83ZuZLrP3qv-XyX1faVL1U08R40IXSr3yZhyx0dfEQS4h-A5ef9AiZIZQy_lLpxQu3-1wPmFVQYRslb35r490aA4nQ&q=concon&sa=X&ved=2ahUKEwj97Zzy4fmRAxUjH7kGHbxqMfQQtKgLegQIERAB&biw=2560&bih=1318) 

## 📄 License

MIT Licensed.
