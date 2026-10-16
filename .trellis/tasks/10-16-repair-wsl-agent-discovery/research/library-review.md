# Library discovery and writeback review

## Missing environment identity in shared skill discovery

- `engine/internal/library/skills.go:434`: sharedSkillsDir() resolves only the Windows engine user's home.
- `skills.go:1671-1686`: foundSkills starts with the engine library and that one shared root, then only each target's primary Skills directory. WSL `~/.agents/skills` is not added for Debian Codex/Grok.
- `skills.go:1673,1697-1736`: inventory groups by skill name across targets. Different content can be represented as Others, but a single Shared string and name-only ImportSkill cannot identify an independent Windows versus WSL source unambiguously once multiple shared roots are discovered.
- `engine/internal/library/targets.go:744-767`: readsShared uses unsuffixed IDs and one host shared root; WSL suffixed IDs cannot acquire the same environment-local shared-root semantics by simply appending another scan directory.
- `targets.go:576-624`: explicit WSL targets cover Claude, Codex, Pi and Omo; an ownFolder allowlist covers others. Unsupported kinds return nil. `omp#profile` is not normalized to omp before allowlist lookup, so its discovered agent row has no library target.

## Existing import can move physical ownership

`skills.go:1748-1752,1781-1835`: import by name chooses the found row, links external/shared sources but otherwise moves a native agent folder into the Windows library. WSL targets receive copies (`targets.go:581`), not a Windows junction. Therefore making WSL-native files visible is not sufficient to guarantee that importing and later editing preserves native ownership or reflects WSL edits.

Planning must explicitly resolve physical source ownership and conflict handling. Do not silently choose the first Windows skill when a same-named WSL variant exists; do not migrate WSL directories into Windows as a side effect of browsing or discovery.

## Existing prompt/instruction handling

`engine/internal/library/library.go:84-86` supports linked source files; the UI explains managed blocks and preserves user-owned text. Parent architecture uses one library plus native target files, not a general two-way synchronization contract. WSL-native local edits and managed content need clear ownership rather than automatic overwrite.

## MCP failure evidence and mechanism

Read-only scout WslLibrary findings corroborated by current runtime metadata:

- `engine/internal/gui/library.go:17-51`: lastProblems stores the last sync result. Glance reattaches it without writing. The Skills banner is not evidence that the skill scan itself failed.
- `engine/internal/gui/assets/plugins/library/client.js:577-580`: any non-own problem shows the Library write banner; Try again triggers all/sync.
- `engine/internal/library/mcp_wsl.go:34-39`: Windows executable commands are refused for WSL targets (except the existing magpie-image special handling). Do not bypass this safety rule or blindly strip .exe.
- Library UI new/market MCP selection defaults to all MCP-capable agents (`client.js:67,2012,5033`), but `reaches` at line 1411 does not filter Windows-program incompatibility. Incompatible WSL assignments are selected before the backend rejects them.
- WSL Pi has no MCP target; named omp profile is absent from the library targets. Capability gaps must be explicit, not inferred from a detected agent row.
- `engine/internal/edit/file.go:135-141`: atomic writes create the temp file in the destination directory. Root-owned Debian `.hermes` mode 755 denies writes by its default user. Changing transport alone cannot repair ownership.

## Safe repair constraints

- Preserve Linux ownership and user settings. No chmod/chown/sudo, silent overwrite or fake success.
- Show target-specific incompatibility and permission evidence, not an undifferentiated retry banner.
- For Windows-specific stdio commands, choose only compatible targets by default. A Linux command must be explicitly available and validated for a target before using it; no automatic executable renaming.
- Treat supported capabilities, installed binary, configuration-only presence, unavailable distro, unreadable directory and write permission as separate signals.
