# PRD — Windows/WSL unified skill + MCP config

Blocked on 11-12-sync-magpie-2611 (upstream library changes may move the design).

## Problem
Skills and MCP configured on Windows mostly fail in WSL and vice versa. A library entry holds one command/path; `library/mcp_wsl.go side()` refuses any Windows-path command except magpie's image server, hooks only map `C:\` -> `/mnt/c`. Today three sources coexist:
- WSL `~/.agents/agents.yaml` + `agents-sync` (14 skills, per-CLI MCP override) — works inside WSL.
- Windows `C:\Users\Administrator\.agents` (38 skills, `servers/*.json`).
- AgentDock (magpie) library.

## Decisions (user, 2026-11-12)
- D1 SSOT = AgentDock library. `agents-sync` retires once parity is reached.
- D2 Skills: one shared source dir; every environment links/copies it; per-environment exclude list.
- D3 magpie upstream sync goes first.

## Requirements (draft)
- An MCP server entry can carry per-environment overrides (windows, wsl:<distro>) for command/args/env, analogous to agents.yaml `override.<cli>`.
- Skills: one source, per-env exclude; status per env visible in UI.
- Import existing WSL agents.yaml and Windows `.agents` into the library without loss.
- Secrets (`${VAR}`) resolved per environment.

## Open
- Where the shared skill dir lives (Windows side vs WSL side) and link vs copy across `\\wsl.localhost`.
- Migration/retirement path for agents-sync and CLAUDE.md references to it.

## Acceptance (draft)
- Same library entry yields a working MCP in Windows Claude/Codex and WSL Claude/Codex.
- Skill set identical across envs except declared excludes.
