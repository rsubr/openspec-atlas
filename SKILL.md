---
name: openspec-atlas-create
description: Scans one or more directories with openspec-atlas and generates OpenSpec-compatible architecture documentation. Use when asked to generate, create, or update OpenSpec docs for a project.
allowed-tools: Bash Read Write
argument-hint: [dir ...]
---

Generate OpenSpec architecture documentation for a codebase.

## Step 1 — Scan the codebase

Run openspec-atlas to produce `structure.json`.

If arguments were provided, scan those directories:
```
./run.sh -o structure.json $ARGUMENTS
```

If no arguments were provided, scan the current directory:
```
./run.sh -o structure.json
```

## Step 2 — Read structure.json

Read the full contents of `structure.json`. This is your input for Step 3.

## Step 3 — Generate documentation

Using the contents of `structure.json` as input, generate OpenSpec-compatible architecture documentation for AI agents.

### Input schema

The JSON contains:
- `files` — source files with their language, namespace, and symbols
- Each symbol has a `name`, `kind`, `line`, and optional `children`

This schema is language-agnostic. Do NOT assume any language-specific behavior.

### Output files

You MUST produce ALL of the following:
1. `openspec/project.md`
2. `openspec/specs/system.md`
3. `openspec/specs/architecture.md`
4. `openspec/specs/modules/<module>.md` — one per logical module
5. `openspec/specs/packages/<package>.md` — one per package, if applicable

### Writing rules (STRICT)

Focus ONLY on:
- structure
- responsibilities
- relationships

DO NOT include:
- code snippets
- method implementations
- low-level details

- Be concise and deterministic
- Use bullet points where possible
- Avoid repetition across files
- Prefer clarity over completeness

### Interpretation rules

- Treat each logical grouping of files as a module
- Treat each symbol as a component — class, function, or service
- Use `kind` to infer role:
  - `class` / `struct` / `interface` → type definition
  - `function` / `method` → behaviour
  - `trait` / `protocol` → contract
- Use namespace/package groupings to infer architecture layers
- If relationships are missing: infer minimal logical connections ONLY if obvious
- If data is incomplete: generate partial but valid documentation

### Content guidelines

**project.md**
- Purpose
- Tech stack (infer from languages present in structure.json)
- Architecture style
- Key modules
- Constraints

**system.md**
- System overview
- Modules
- Entry points
- External systems (if inferable)

**architecture.md**
- Architecture style
- Layers
- Component interaction
- Data flow (high-level only)

**module docs**
- Responsibility
- Key components
- Dependencies
- Notes

**package docs**
- Responsibility
- Classes/components
- Dependencies

### Output format (VERY IMPORTANT)

Produce ALL files in a single response using this exact format:

```
=== FILE: openspec/project.md ===
<content>

=== FILE: openspec/specs/system.md ===
<content>

=== FILE: openspec/specs/architecture.md ===
<content>

=== FILE: openspec/specs/modules/<module>.md ===
<content>

=== FILE: openspec/specs/packages/<package>.md ===
<content>
```

### Quality bar

The output must allow an AI agent to:
- Understand system structure WITHOUT reading code
- Navigate modules confidently
- Avoid re-analysing source files

### Constraints

- Each file must be under ~200 lines
- No duplication of entire sections across files
- Maintain consistent naming across all files

## Step 4 — Write files

For every `=== FILE: <path> ===` block produced in Step 3:
- Create any missing parent directories
- Write the file content to that exact path

Do not skip any file. Do not modify paths.
