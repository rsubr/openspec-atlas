# openspec-atlas

Static code scanner that extracts the structure of a codebase into a single JSON file.

Part of the OpenSpec pipeline:

```
Codebase → openspec-atlas → structure.json → LLM Summarization → OpenSpec Docs
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
./run.sh

# Scan one or more directories
./run.sh /path/to/repo
./run.sh /path/to/repo1 /path/to/repo2

# Custom output file
./run.sh -o output.json /path/to/repo

# Ignore .gitignore files and scan everything
./run.sh -all /path/to/repo
```

Or use the binary directly:

```bash
./dist/openspec-atlas-linux-x86_64 [-o output.json] [-all] <dir> [dir ...]
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

## Building

Requires Go 1.21+ and `aarch64-linux-gnu-gcc` for the ARM64 cross-build.

```bash
./build.sh
```

Produces:

```
dist/openspec-atlas-linux-x86_64   # statically linked, stripped
dist/openspec-atlas-linux-arm64    # statically linked, stripped
```

`run.sh` auto-detects the current OS and architecture and selects the correct binary.

---

## Architecture

Built on [tree-sitter](https://tree-sitter.github.io/tree-sitter/) via [go-tree-sitter](https://github.com/smacker/go-tree-sitter). Each language is defined by:

- A set of S-expression queries that capture `@name` (the identifier) and `@decl` (the full declaration node)
- A namespace query for package/module detection
- A flag per query indicating whether the symbol is a container (can own children)

Hierarchy is resolved by comparing byte ranges from `@decl` captures — no language-specific logic needed in the extraction engine.

---

## Roadmap

- Incremental updates (diff-based rescanning)
- Spring Boot annotation detection and layer classification
- API endpoint extraction
- Integration with OpenSpec MCP tooling
