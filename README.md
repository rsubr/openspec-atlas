# openspec-atlas

`openspec-atlas` scans a legacy codebase — human-written, any language — and produces a structured atlas of its architecture. The atlas is a single JSON file that AI agents can consume directly, without reading source code.

A `SKILL.md` is included in this repo. It is an agent skill that automates the full pipeline: scan the codebase, then generate OpenSpec-compatible architecture documentation from the atlas. **After installing the binary, the skill must be manually registered with your AI agent.** See [Installing the Agent Skill](#installing-the-agent-skill) below.

Supported agents: **Claude Code**, **OpenAI Codex CLI**, **Gemini CLI**.

---

## Pipeline

```
Legacy Codebase → openspec-atlas → atlas.json → Agent Skill → OpenSpec Docs
```

---

## Problem

LLM agents repeatedly read entire codebases, waste tokens, and reconstruct understanding every session. This leads to high cost, slow responses, and inconsistent reasoning.

---

## Solution

Run `openspec-atlas` once against a codebase. It produces a structured JSON map of every file, namespace, type, and symbol. Agents consume the JSON instead of the source — no repeated parsing, no wasted tokens.

**Generate once. Reuse forever.**

---

## What It Extracts

For each source file:

- **Language** — detected from file extension
- **Namespace** — package, module, or namespace declaration
- **Types** — classes, structs, interfaces, enums, traits, objects
- **Symbols** — functions and methods, nested under their parent type
- **Annotations** — decorators and attributes (`@GetMapping`, `@Injectable`, `[Authorize]`, etc.)
- **Endpoints** — fully-resolved HTTP routes for Spring Boot (Java/Kotlin) and ASP.NET (C#)

Across the whole project (not per-file):

- **Environment variables** — all `process.env.X`, `os.Getenv()`, `os.environ[]`, `ENV[]` references; `.env` file definitions; tracks which files use each var and whether a default is provided
- **HTTP edges** — outgoing `fetch()` / `axios` / `ky` / `got` calls in JS/TS matched against backend route handlers (confidence: exact / path / fuzzy)
- **Database schema models** — tables and fields from Prisma `model` blocks, SQL `CREATE TABLE`, SQLAlchemy `Base` subclasses, TypeORM `@Entity` classes, and GORM-tagged Go structs
- **Middleware** — Express `app.use()` registrations (helmet, cors, jwt, passport, morgan, rateLimit…), NestJS `@UseGuards` / `@UseInterceptors` / `@UsePipes`, FastAPI `Depends()`; categorised by type
- **UI components** — React PascalCase components (`.tsx`/`.jsx`) with props, Svelte `.svelte` files with `export let` props, Angular `@Component` classes with `@Input` props, Vue SFCs

---

## Supported Languages

| Language | Extensions |
|---|---|
| Java | `.java` |
| Go | `.go` |
| Python | `.py` |
| TypeScript | `.ts` |
| TSX | `.tsx` |
| JavaScript | `.js` `.mjs` `.cjs` |
| Rust | `.rs` |
| C | `.c` `.h` |
| C++ | `.cpp` `.cc` `.cxx` `.hpp` |
| C# | `.cs` |
| Ruby | `.rb` |
| Kotlin | `.kt` `.kts` |
| Scala | `.scala` |
| Swift | `.swift` |
| PHP | `.php` |
| Lua | `.lua` |
| Bash | `.sh` `.bash` |
| Vue SFC | `.vue` |

---

## Output Format

The top-level JSON object has the following sections (sections with no results are omitted):

```json
{
  "files": [...],
  "env_vars": [...],
  "http_edges": [...],
  "schema_models": [...],
  "middleware": [...],
  "ui_components": [...]
}
```

### files

```json
{
  "path": "src/auth/service.go",
  "language": "go",
  "namespace": "auth",
  "symbols": [
    { "name": "AuthService", "kind": "struct", "line": 12 },
    { "name": "Login",       "kind": "function", "line": 28 }
  ]
}
```

Types that contain methods have a `children` array. Symbols with framework annotations carry an `annotations` array, and HTTP handlers carry an `endpoint` object:

```json
{
  "name": "UserController",
  "kind": "class",
  "line": 10,
  "annotations": [{ "name": "RestController" }],
  "children": [
    {
      "name": "getUser",
      "kind": "method",
      "line": 18,
      "annotations": [{ "name": "GetMapping", "value": "/{id}" }],
      "endpoint": { "method": "GET", "path": "/users/{id}" }
    }
  ]
}
```

### env_vars

```json
{ "name": "DATABASE_URL", "files": ["src/db/connect.ts"], "has_default": false, "required": true }
```

### http_edges

```json
{ "caller_file": "src/client/api.ts", "caller_line": 42, "method": "POST", "path": "/users", "confidence": "exact", "handler_file": "src/users/controller.ts" }
```

### schema_models

```json
{ "name": "User", "file": "prisma/schema.prisma", "line": 5, "orm": "prisma",
  "fields": [
    { "name": "id",    "type": "Int",    "nullable": false },
    { "name": "email", "type": "String", "nullable": false }
  ]
}
```

`orm` is one of: `prisma`, `sql`, `sqlalchemy`, `typeorm`, `gorm`.

### middleware

```json
{ "name": "helmet", "type": "auth", "framework": "express", "file": "src/server.ts", "line": 8 }
```

`type` is one of: `auth`, `cors`, `rate-limit`, `validation`, `logging`, `error-handler`, `custom`.

### ui_components

```json
{ "name": "UserCard", "framework": "react", "file": "src/components/UserCard.tsx", "line": 12, "props": ["userId", "onSelect"] }
```

`framework` is one of: `react`, `vue`, `svelte`, `angular`.

---

## Usage

```bash
# Scan current directory → structure.json
openspec-atlas .

# Scan one or more directories
openspec-atlas /path/to/repo
openspec-atlas /path/to/repo1 /path/to/repo2

# Custom output file
openspec-atlas -o output.json /path/to/repo

# Ignore .gitignore files and scan everything
openspec-atlas -all /path/to/repo
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-o` | `structure.json` | Output file path |
| `-all` | off | Bypass `.gitignore` and process all files |
| `-version` | — | Print version and exit |

---

## Drift Detection

`openspec-atlas drift` compares a saved baseline atlas against a fresh scan and reports what changed — useful for tracking spec drift in CI or during code review.

```bash
# 1. Save a baseline at a known-good point (e.g. on main before a PR)
openspec-atlas -o baseline.json .

# 2. After making changes, check what drifted
openspec-atlas drift --baseline baseline.json .

# Compare two pre-saved JSON files without re-scanning
openspec-atlas drift --baseline baseline.json --current current.json

# Machine-readable output for CI
openspec-atlas drift --baseline baseline.json --json .

# Fail with exit code 1 if any symbols were removed
openspec-atlas drift --baseline baseline.json --fail-on removed .
```

Example output:

```
[endpoint]
  - removed  DELETE /users/{id}  (src/users/handler.go)

[env_var]
  + added    JWT_SECRET

[symbol]
  ~ changed  UserService  — kind changed: struct → interface  (src/users/service.go)
  + added    AuditLogger  — struct  (src/audit/logger.go)

summary: +2 added  -1 removed  ~1 changed  (4 total)
```

Drift is detected across all output sections: symbols, endpoints, env vars, schema models, middleware, and UI components.

### Drift Flags

| Flag | Default | Description |
|---|---|---|
| `--baseline` | — | Path to baseline `structure.json` (required) |
| `--current` | — | Path to current `structure.json`; re-scans dirs if absent |
| `--json` | off | Emit machine-readable JSON report |
| `--fail-on` | `removed` | Exit 1 if any issue of this kind exists; must be one of `added`, `removed`, `changed`, `none` — an invalid value is rejected with an error |
| `--all` | off | Bypass `.gitignore` when re-scanning |

---

## Gitignore Support

By default, `openspec-atlas` respects `.gitignore` files at every level of the directory tree. Ignored directories are skipped before being entered. The `.git` directory is always excluded.

Pass `-all` to disable this and scan everything.

---

## Installation

Download the binary for your platform from the [releases page](https://github.com/rsubr/openspec-atlas/releases) and place it in your PATH:

```bash
# Linux x86_64
sudo cp openspec-atlas-linux-x86_64 /usr/local/bin/openspec-atlas
sudo chmod +x /usr/local/bin/openspec-atlas

# Linux arm64
sudo cp openspec-atlas-linux-arm64 /usr/local/bin/openspec-atlas
sudo chmod +x /usr/local/bin/openspec-atlas
```

---

## Building from Source

Requires Go 1.22+ and `aarch64-linux-gnu-gcc` for the ARM64 cross-build.

```bash
./build.sh
```

Produces:

```
dist/openspec-atlas-linux-x86_64   # statically linked, stripped
dist/openspec-atlas-linux-arm64    # statically linked, stripped
```

---

## Architecture

Built on [tree-sitter](https://tree-sitter.github.io/tree-sitter/) via [go-tree-sitter](https://github.com/smacker/go-tree-sitter). Each language is defined by:

- A set of S-expression queries that capture `@name` (the identifier) and `@decl` (the full declaration node)
- A namespace query for package/module detection
- A flag per query indicating whether the symbol is a container (can own children)

Hierarchy is resolved by comparing byte ranges from `@decl` captures — no language-specific logic needed in the extraction engine.

---

## Internal Structure

All logic lives in the `internal/` package. Key files:

| File | Responsibility |
|---|---|
| `cli.go` | Entry point; subcommand dispatch map; flag parsing |
| `scan.go` | `walkSourceFiles` (directory walk + parse) and `scanProjects` (orchestrator) |
| `parse.go` | Tree-sitter file parsing, namespace extraction, symbol extraction |
| `annotations.go` | Decorator/attribute extraction from parse tree nodes |
| `hierarchy.go` | Assigns flat raw symbols into a nested parent/child tree |
| `drift.go` | `drift` subcommand: generic `diffByKey` helper, per-category diff functions, `runDrift` |
| `config.go` | `LanguageConfig` type; compiled tree-sitter query registry |
| `languages.go` | Per-language configuration (queries, extensions, post-processors) |
| `endpoints.go` | Spring Boot / ASP.NET endpoint resolution post-processor |
| `envvars.go` | Environment variable extraction across source and `.env` files |
| `httpedges.go` | Outgoing HTTP call detection and backend route matching |
| `dbschema.go` | Schema model extraction (Prisma, SQL, SQLAlchemy, TypeORM, GORM) |
| `middleware.go` | Middleware registration detection (Express, NestJS, FastAPI) |
| `uicomponents.go` | UI component detection (React, Svelte, Angular, Vue) |
| `exthelpers.go` | Shared utilities: file I/O, path helpers, line indexing, string utilities |
| `ignore.go` | `.gitignore` evaluation with directory-level caching |
| `vue.go` | Vue SFC `<script>` block extractor |

### Design notes

**Subcommand dispatch** — `cli.go` uses a `subcommands` map (`map[string]func([]string, io.Writer, io.Writer) error`). Adding a new subcommand is a one-line registration.

**Generic diff** — `drift.go` exposes `diffByKey[T any](baseline, current []T, key, removed, added, changed)`. All four flat-list diff functions (`diffEnvVars`, `diffSchemaModels`, `diffMiddleware`, `diffUIComponents`) delegate to this helper; only `diffSymbols` and `diffEndpoints` use hand-rolled maps because they operate on pre-flattened intermediate representations.

**Collector pattern** — Every extended analyser (`collectEnvVars`, `collectHTTPEdges`, `collectSchemaModels`, `collectMiddleware`, `collectUIComponents`) takes the same `(allPaths []string, files []FileInfo, displayRoot string)` signature. `scanProjects` calls them all after the walk completes.

---

## Installing the Agent Skill

`SKILL.md` defines a four-step pipeline:

1. Run `openspec-atlas` to produce `structure.json`
2. Read `structure.json`
3. Generate OpenSpec documentation (project, system, architecture, module, and package docs)
4. Write all files to the `openspec/` directory

The binary must be installed and available in your `PATH` before registering the skill.

---

### Claude Code

Copy `SKILL.md` into your Claude skills directory. The skill can be installed globally (available in all projects) or per-project.

**Global install:**
```bash
mkdir -p ~/.claude/skills/openspec-atlas-create
cp SKILL.md ~/.claude/skills/openspec-atlas-create/SKILL.md
```

**Project install:**
```bash
mkdir -p .claude/skills/openspec-atlas-create
cp SKILL.md .claude/skills/openspec-atlas-create/SKILL.md
```

Invoke in any Claude Code session:
```
/openspec-atlas-create
/openspec-atlas-create /path/to/repo
/openspec-atlas-create /path/to/repo1 /path/to/repo2
```

---

### OpenAI Codex CLI

Codex reads instructions from an `AGENTS.md` file. Append the skill body to your global or project-level instructions file.

**Global install:**
```bash
cat SKILL.md >> ~/.codex/AGENTS.md
```

**Project install:**
```bash
cat SKILL.md >> AGENTS.md
```

Then ask Codex:
```
Run openspec-atlas on this repo and generate OpenSpec documentation.
```

---

### Gemini CLI

Gemini CLI reads instructions from a `GEMINI.md` file. Append the skill body to your global or project-level file.

**Global install:**
```bash
cat SKILL.md >> ~/.gemini/GEMINI.md
```

**Project install:**
```bash
cat SKILL.md >> GEMINI.md
```

Then ask Gemini:
```
Run openspec-atlas on this repo and generate OpenSpec documentation.
```

---

## Roadmap

- Incremental updates (diff-based rescanning)
- Call graph with fanIn/fanOut metrics
- Duplicate code detection
- Refactor priority scoring
- Integration with OpenSpec MCP tooling
