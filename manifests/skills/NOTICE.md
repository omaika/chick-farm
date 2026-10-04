# Skill sources

The skills in this directory are adapted from the skills of seatworks
(`omaika/seatworks` on GitHub, `plugin/content/skills/`), under the MIT license below.
The adaptation replaced seatworks' tools, records and paths with piggery's: work is given and
handed back by mail (`send`, kinds `task`, `handback`, `ask`), workers are started and stopped with
`agent`, a lane's plan is pinned on the team board, and the team's records live at the repository's
root (`CONTEXT.md`, `NOTEBOOK.md`, `notes/`). The flat layout (one directory per skill) replaces
seatworks' sets by role. `ultra-review` was not taken.

| Skill | In seatworks | Its own sources, as seatworks' NOTICE.md gives them |
|---|---|---|
| `council` | `lead/council` | SLP material (`council/`, `report-format.md`) |
| `repo-refresh` | `lead/repo-refresh` | SLP material (`repo-refresh/`) |
| `planning-lanes` | `lead/planning-lanes` | written for seatworks |
| `test-proof-debt-audit` | `peer/test-proof-debt-audit` | SLP material (`test-proof-debt-audit-SKILL.md`, `catalog.md`) |
| `test-first` | `peer/test-first` | obra/superpowers `test-driven-development`; mattpocock/skills `tdd`; SLP material and talk |
| `diagnosing-bugs` | `peer/diagnosing-bugs` | mattpocock/skills `diagnosing-bugs`; obra/superpowers `systematic-debugging` |
| `security-check` | `peer/security-check` | addyosmani/agent-skills `security-and-hardening`; trailofbits/skills `sharp-edges` (ideas only) |
| `architecture-premise-audit` | `supervisor/architecture-premise-audit` | SLP material (`SKILL.md`, `structural-antipatterns.md`) |
| `grilling` | `supervisor/grilling` | mattpocock/skills `grilling` and `domain-modeling`; `references/context-format.md` written for piggery |
| `pre-mortem` | `supervisor/pre-mortem` | Gary Klein's project premortem (ideas only); SLP `council` |
| `retrospective` | `supervisor/retrospective` | Cemri et al., "Why Do Multi-Agent LLM Systems Fail?" (ideas only); the SLP author's talk; `references/notebook-format.md` from seatworks' `records/NOTEBOOK.md` |

"SLP material" is a practitioner's set of prompts and skills that seatworks used with its owner's
agreement; its use in piggery was confirmed with that owner as well.

Upstream sources: obra/superpowers (MIT, © 2025 Jesse Vincent), mattpocock/skills (MIT, © 2026 Matt
Pocock), addyosmani/agent-skills (MIT, © 2025 Addy Osmani), trailofbits/skills (CC-BY-SA-4.0, ideas
only, no text taken).

## seatworks license

```
MIT License

Copyright (c) 2026 long7400

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
