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

---

## Output Format

```json
{
  "files": [
    {
      "path": "src/auth/service.go",
      "language": "go",
      "namespace": "auth",
      "symbols": [
        {
          "name": "AuthService",
          "kind": "struct",
          "line": 12
        },
        {
          "name": "Login",
          "kind": "function",
          "line": 28
        }
      ]
    }
  ]
}
```

Types that contain methods have a `children` array:

```json
{
  "name": "UserController",
  "kind": "class",
  "line": 10,
  "children": [
    { "name": "getUser",   "kind": "method", "line": 18 },
    { "name": "createUser","kind": "method", "line": 34 }
  ]
}
```

---

## Usage

```bash
# Scan current directory → structure.json
openspec-atlas

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

Requires Go 1.21+ and `aarch64-linux-gnu-gcc` for the ARM64 cross-build.

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
- Spring Boot annotation detection and layer classification
- API endpoint extraction
- Integration with OpenSpec MCP tooling
