# Domain Docs

How engineering skills should consume this repository’s domain documentation when exploring the codebase.

## Before exploring, read these

- `CONTEXT.md` at the repository root.
- `docs/adr/` entries relevant to the area being changed.

If these files do not exist, proceed silently. Do not recommend creating them preemptively. The domain-modeling workflows create them when terminology or architectural decisions are actually resolved.

## File structure

This repository uses a single-context layout:

```text
/
├── CONTEXT.md
├── docs/
│   └── adr/
├── cmd/
└── internal/
```

## Use the glossary’s vocabulary

When output names a domain concept—in an issue, refactor proposal, hypothesis, or test—use the term defined in `CONTEXT.md`. Do not drift to synonyms the glossary explicitly avoids.

If a necessary concept is absent, reconsider whether the project uses a different term or note the gap for domain modeling.

## Flag ADR conflicts

If proposed work contradicts an existing ADR, surface the conflict explicitly instead of silently overriding the decision.
