# Manque AI - SOTA Code Reviewer Roadmap

## Vision
Transform Manque AI from a single-shot PR reviewer into a stateful, context-aware, interactive code review assistant that rivals CodeRabbit.

---

## Phase 1: Quick Wins (1-2 weeks effort)
*High impact, low complexity improvements*

### 1.1 GitHub Suggested Changes Format
**Impact:** High | **Effort:** Low

Replace plain code suggestions with GitHub's native suggestion syntax:
```markdown
```suggestion
const result = await fetch(url).catch(handleError);
```​
```

**Files to modify:**
- `pkg/ai/prompts.go` - Update prompt to request suggestion format
- `pkg/ai/types.go` - Add `SuggestedCode` field to Comment
- `cmd/root.go` - Format comments with suggestion blocks

**Acceptance:** Users can click "Commit suggestion" directly on GitHub

---

### 1.2 Review Action Types (REQUEST_CHANGES / APPROVE)
**Impact:** High | **Effort:** Low

Currently hardcoded to `COMMENT`. Add logic:
- Critical issues → `REQUEST_CHANGES`
- Score > 90, no criticals → `APPROVE`
- Otherwise → `COMMENT`

**Files to modify:**
- `pkg/github/client.go:313` - Dynamic event type
- `pkg/ai/types.go` - Add `ShouldBlock bool` to ReviewResult

---

### 1.3 Configuration File Support
**Impact:** High | **Effort:** Medium

Add `.manque.yml` configuration:
```yaml
version: 1
review:
  auto_approve_threshold: 90
  block_on_critical: true

ignore:
  - "**/*.generated.go"
  - "vendor/**"
  - "*.lock"

rules:
  - path: "src/tests/**"
    severity_override: suggestion  # Never block for test files
  - path: "src/api/**"
    extra_rules: |
      - All endpoints must have OpenAPI documentation
      - Authentication required for all routes
```

**New files:**
- `pkg/config/file.go` - YAML config loader
- `internal/config.go` - Merge file config with env vars

---

### 1.4 Smarter Diff Chunking
**Impact:** Medium | **Effort:** Low

Instead of truncating at 100KB, split into logical chunks:
- Review files in batches
- Aggregate comments
- Prioritize changed files by complexity

**Files to modify:**
- `pkg/review/engine.go` - Add chunked review logic
- `pkg/diff/parser.go` - Add file-level size estimation

---

## Phase 2: Context Expansion (2-3 weeks effort)
*Give the LLM visibility beyond the diff*

### 2.1 Referenced File Fetching
**Impact:** Very High | **Effort:** Medium

When a diff imports/calls something, fetch that file:
```go
// If diff adds: import { validateUser } from './auth'
// Fetch and include ./auth.ts in context
```

**Implementation:**
1. Parse imports from diff (regex per language)
2. Resolve relative paths
3. Fetch file contents via GitHub API or local fs
4. Include relevant snippets in LLM context

**New files:**
- `pkg/context/resolver.go` - Import/dependency resolver
- `pkg/context/fetcher.go` - File content fetcher

**Files to modify:**
- `pkg/review/engine.go` - Inject expanded context

---

### 2.2 Function Definition Lookup
**Impact:** High | **Effort:** Medium

When reviewing a function call, fetch the function definition:
```go
// Diff calls: result := processOrder(order)
// Fetch: func processOrder(o Order) Result { ... }
```

**Approach:**
- Use `ctags` or tree-sitter for symbol extraction
- Build symbol index on first run
- Query index during review

**New files:**
- `pkg/symbols/indexer.go` - Symbol table builder
- `pkg/symbols/query.go` - Symbol lookup

---

### 2.3 Git Blame Context
**Impact:** Medium | **Effort:** Low

Include authorship context:
- Who wrote the code being modified?
- How old is this code?
- Has it been stable or frequently changed?

**Files to modify:**
- `pkg/github/client.go` - Add blame API call
- `pkg/review/engine.go` - Include blame in prompt context

---

## Phase 3: Incremental & Stateful Reviews (3-4 weeks effort)
*Remember what's been reviewed*

### 3.1 Per-Commit Incremental Reviews
**Impact:** Very High | **Effort:** High

Track reviewed commits and only analyze new changes:
```
PR opened with commit A → Full review
Push commit B → Only review diff(A..B)
Push commit C → Only review diff(B..C)
```

**Implementation:**
- Store reviewed commit SHAs (GitHub check annotations or comments)
- Calculate incremental diff
- Merge comments intelligently

**New files:**
- `pkg/state/tracker.go` - Commit tracking
- `pkg/diff/incremental.go` - Commit-range diff

---

### 3.2 Comment Threading & Updates
**Impact:** High | **Effort:** Medium

Instead of duplicate comments, update existing ones:
- Track comment IDs per file+line
- Update body if suggestion changes
- Mark as resolved if issue fixed

**Files to modify:**
- `pkg/github/client.go` - Add comment update logic
- Add persistent comment mapping (GitHub issue body or external store)

---

### 3.3 Session Memory
**Impact:** High | **Effort:** High

Remember context across review rounds:
- Previous suggestions made
- User responses/dismissals
- Conversation history

**Options:**
1. Store in PR body (hidden HTML comments)
2. Store in GitHub check run annotations
3. External database (Redis/SQLite)

**New files:**
- `pkg/state/session.go` - Session management
- `pkg/state/storage.go` - Persistence layer

---

## Phase 4: Interactive Conversations (2-3 weeks effort)
*Two-way communication*

### 4.1 @mention Command Parsing
**Impact:** Very High | **Effort:** Medium

Respond to user commands in PR comments:
```
@manque explain this function
@manque suggest a fix
@manque ignore this warning
@manque regenerate review
```

**Implementation:**
- GitHub webhook for `issue_comment` events
- Command parser
- Context-aware response generation

**New files:**
- `pkg/commands/parser.go` - Command extraction
- `pkg/commands/handlers.go` - Command implementations
- `cmd/webhook.go` - Webhook server mode

---

### 4.2 Conversation Context
**Impact:** High | **Effort:** Medium

Include previous conversation in LLM context:
```
User: "Why is this a problem?"
Bot: [Generates explanation with full context of the original comment]
```

**Files to modify:**
- `pkg/ai/types.go` - Add conversation history
- `pkg/ai/prompts.go` - Conversation-aware prompts

---

### 4.3 Feedback Learning
**Impact:** Medium | **Effort:** High

Track user reactions to improve:
- Thumbs up/down on comments
- Comments marked as resolved vs dismissed
- Suggestions accepted vs ignored

**Storage:** Analytics DB or GitHub discussions

---

## Phase 5: Deep Code Understanding (4-6 weeks effort)
*AST and semantic analysis*

### 5.1 Tree-sitter Integration
**Impact:** Very High | **Effort:** High

Parse code into AST for:
- Accurate symbol extraction
- Scope analysis
- Type inference (where possible)

**Languages to support:**
1. Go, TypeScript, JavaScript (priority)
2. Python, Rust, Java (secondary)

**New files:**
- `pkg/ast/parser.go` - Multi-language AST parsing
- `pkg/ast/symbols.go` - Symbol extraction
- `pkg/ast/scope.go` - Scope analysis

---

### 5.2 Cross-File Impact Analysis
**Impact:** Very High | **Effort:** High

Detect when changes break other files:
```
"Renaming `getUserId()` to `getUserID()` will break:
 - src/handlers/auth.go:45
 - src/services/user.go:112"
```

**Implementation:**
- Build call graph
- Track symbol usages
- Compare before/after states

---

### 5.3 Breaking Change Detection
**Impact:** High | **Effort:** Medium

For public APIs:
- Function signature changes
- Removed exports
- Type changes
- Deprecation additions

**New files:**
- `pkg/breaking/detector.go` - API diff analysis
- `pkg/breaking/report.go` - Breaking change report

---

## Phase 6: External Tool Integration (2-3 weeks effort)
*Aggregate signals from existing tools*

### 6.1 Linter Output Ingestion
**Impact:** High | **Effort:** Medium

Parse and incorporate:
- ESLint/Prettier (JS/TS)
- golangci-lint (Go)
- Ruff/Flake8 (Python)
- Clippy (Rust)

**Implementation:**
- Run linters in CI
- Parse SARIF/JSON output
- Merge with AI comments (dedupe)

**New files:**
- `pkg/linters/runner.go` - Linter execution
- `pkg/linters/parsers/` - Per-tool parsers
- `pkg/linters/merger.go` - Comment deduplication

---

### 6.2 Security Scanner Integration
**Impact:** Very High | **Effort:** Medium

Integrate:
- Semgrep (SAST)
- Trivy (dependency vulns)
- Gitleaks (secrets)

**Output:** Security findings with CVE references

---

### 6.3 Test Coverage Overlay
**Impact:** Medium | **Effort:** Low

Show coverage impact:
- "This new function has 0% test coverage"
- "Coverage dropped from 85% to 82%"

**Files to modify:**
- `pkg/review/engine.go` - Include coverage data
- Add coverage report parser

---

## Phase 7: Analytics & Learning (ongoing)
*Continuous improvement*

### 7.1 Review Quality Metrics
- Time from PR open to first review
- Comment acceptance rate
- False positive rate
- Most common issue categories

### 7.2 Team Patterns
- Common mistakes by team/author
- Trending issues over time
- Knowledge gaps identification

### 7.3 Model Fine-tuning Data
- Collect accepted suggestions as training data
- Build dataset for potential fine-tuning
- A/B test prompt variations

---

## Implementation Priority Matrix

| Phase | Feature | Impact | Effort | Priority |
|-------|---------|--------|--------|----------|
| 1.1 | GitHub Suggestions | High | Low | P0 |
| 1.2 | Review Actions | High | Low | P0 |
| 1.3 | Config File | High | Medium | P0 |
| 2.1 | Referenced Files | Very High | Medium | P0 |
| 4.1 | @mention Commands | Very High | Medium | P1 |
| 3.1 | Incremental Reviews | Very High | High | P1 |
| 5.1 | Tree-sitter AST | Very High | High | P1 |
| 2.2 | Function Lookup | High | Medium | P2 |
| 3.2 | Comment Threading | High | Medium | P2 |
| 6.1 | Linter Integration | High | Medium | P2 |
| 6.2 | Security Scanners | Very High | Medium | P2 |
| 5.2 | Cross-File Analysis | Very High | High | P3 |
| 3.3 | Session Memory | High | High | P3 |
| 4.3 | Feedback Learning | Medium | High | P4 |

---

## Recommended Starting Point

**Sprint 1 (Week 1-2):**
1. ✅ GitHub Suggested Changes format
2. ✅ Review action types (APPROVE/REQUEST_CHANGES)
3. ✅ Basic `.manque.yml` config file

**Sprint 2 (Week 3-4):**
1. ✅ Referenced file fetching (imports)
2. ✅ Smarter diff chunking

**Sprint 3 (Week 5-6):**
1. ✅ @mention command support (webhook mode)
2. ✅ Per-commit incremental reviews

This gets you to feature parity with CodeRabbit's core value prop in ~6 weeks.

---

## Success Metrics

| Metric | Current | Target (3mo) | Target (6mo) |
|--------|---------|--------------|--------------|
| Comment acceptance rate | Unknown | 60% | 75% |
| False positive rate | Unknown | <20% | <10% |
| Context window utilization | ~30% | 70% | 90% |
| Supported interactions | 1 (review) | 5 | 10+ |
| Languages with AST support | 0 | 3 | 6 |

---

## Tech Debt to Address

1. **Duplicate code in engine.go** - `Review()` and `ReviewWithContext()` are nearly identical
2. **No tests for prompts** - Add golden file tests for prompt outputs
3. **Hardcoded limits** - Move magic numbers to config (100KB, 8KB, etc.)
4. **Error handling** - Many errors are wrapped but not typed for handling
5. **No retry logic** - LLM API calls should have exponential backoff
