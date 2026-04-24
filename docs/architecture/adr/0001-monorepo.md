# ADR-0001: Monorepo with Go workspaces

**Status:** Accepted  
**Date:** 2026-04

## Context

Seven backend services, one shared library, proto definitions, a React frontend. Where does it all live?

Options considered:
1. **Polyrepo** — one repo per service
2. **Monorepo, single `go.mod`** — everything under one module
3. **Monorepo with Go workspaces** — each service has its own `go.mod`, root `go.work` links them

## Decision

Option 3 — monorepo with Go workspaces.

## Why

Polyrepo makes cross-service refactors painful (multi-repo PRs, coordinating releases). A single `go.mod` doesn’t let services diverge on dependencies over time. Go workspaces hit the sweet spot: each service is an independent module that compiles in isolation, but `go.work` means you don’t need to publish the shared library to use it locally.

| | Polyrepo | Single module | Go workspaces |
|---|---|---|---|
| Cross-service refactor | painful | easy | easy |
| Independent deps | yes | no | yes |
| Local `replace` directives | required | not needed | handled by go.work |
| Docker build context | small | huge | moderate |

## Consequences

- Atomic commits across services and shared library.
- Each Dockerfile COPYs only `shared/` + the relevant service directory — images stay small.
- `go.work` is not used inside Docker builds. Each service `go.mod` has a `replace` pointing at `../../shared` for the builder stage.
- CI needs `go work sync` before running workspace-wide lint/test.
