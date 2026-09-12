# Design

Design docs capture durable principles and decisions that should outlive a
single pull request.

## Files

- `principles.md` - engineering principles for structure and documentation.
- `proposals/` - design proposals for work not yet built.
- `decisions/` - architecture decision records.

## Proposals and decisions are different documents

A **proposal** argues for work that does not exist yet. It is long, carries
goals, alternatives, a phased implementation plan and open questions, and it
is expected to change while the work is under way.

A **decision record** states what was settled and why, in about a page. It is
durable: an accepted ADR is superseded by a new one rather than edited, so it
must not carry anything that the implementation will later invalidate.

Write the proposal first. When the work ships, document the resulting
behaviour in `concepts/`; the proposal stays as the record of how the design
was arrived at.

An ADR is worth adding only when the rationale has no other home — when the
concept page would otherwise grow a long justification section that is not
about using the feature. Not every shipped proposal needs one, and an ADR
written before the work exists is a proposal wearing the wrong template.

## Proposal Template

```markdown
# PROP-0000: Title

## Status
Draft | Accepted | Implemented | Withdrawn

## Motivation
## Goals
## Non-Goals
## Design
## Alternatives considered
## Risks and trade-offs
## Implementation plan
## Open questions
## References
```

## Decision Record Template

```markdown
# ADR-0000: Title

## Status
Proposed | Accepted | Superseded

## Context
## Decision
## Consequences
## References
```
