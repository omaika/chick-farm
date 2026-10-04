---
name: retrospective
description: "Turns a period of a team's recorded events, mail and git history into notebook rows and at most one proposed change, by classifying each costly episode as a specification, coordination, or verification failure, counting which pattern has been seen twice, and weighing each mechanism by what it changed. Use when the Human asks how a run went, or after an episode that cost a rework round; not for a single surprise."
---

# Retrospective

The rule that matters most: one change per retrospective, with two dated episodes behind it.

You turn what piggery and git already recorded into updated rows of `NOTEBOOK.md` at the repository's root (its shape is in [references/notebook-format.md](references/notebook-format.md); create it from there if it is missing) and at most one change worth making. Multi-agent work mostly fails by its organization rather than by model capability, so the class of each failure says where its fix belongs, and "use a stronger model" is proposed only once you can name the instruction the weaker one dropped. The three classes are MAST's three families of multi-agent failure; name the mode within one when you can, such as a step repeated, a task derailed, or verification skipped or incomplete.

| Class | It looks like | The fix lives in |
|---|---|---|
| Specification | work nobody asked for, an invented contract, a different problem solved | the brief's fields, the prompt that let work start without them |
| Coordination | two writers on one path, a question that died, a result at the wrong agent, mail ignored | the fields of a task layout, a handback or an ask, the template's routing, what each role is told |
| Verification | a proof that passed without the behavior, a summary taken as evidence, a finding after a merge | a brief's check, when a review is started, the checks run before a merge |

## Procedure

1. **Collect evidence, not memory.** Start from what piggery recorded, with the admin commands in your shell (they read and change nothing): `piggery --admin log --team <team> --limit 500` for the events, oldest first (the next page: `--after` the last line's number) (refusals `denied` and `held`, `watch_fired` and `watch_escalated` with the timer that fired, `merge_conflict` and how each merge ended, `task_accepted` and `task_dropped` with the task's `#N`, workers `exited` or `gone`), `piggery --admin mail --team <team> --limit 200` for the mail with its kinds (each `rework`, `ask` and `handback`, and what it answers; page back with `--before`), and `git log` on the branches the work landed on; quote each line you use with its time or `#N`. Never hand whole logs to a model to find the failure: step-level attribution that way is right about one time in seven. A correction the Human made of you counts too: each dated line under Corrections in the notebook is an episode, a corrected summary of their word a specification failure and a corrected status a coordination failure. Keep to these: a retrospective from memory reproduces what you already believed.
2. **Write one episode per costly event:** what happened, its cost in something countable (a rework round, a cut task, a finding after a merge, a question the Human answered twice), and its class. An event with no cost stays in the log.
3. **Count.** Group episodes by class and mechanism, with a count and the dates behind it. "Seen twice" is a count, not an impression.
4. **Weigh each mechanism by what it changed, not by how much it ran.** Count it from the mail and the events: how many of each reviewer's or supervisor's judgments sent work back (`rework`) before the task was accepted, how many tasks were dropped, how many challenges to a premise changed the plan, how many asks went up and about what, how many findings were re-checked. A review role that seldom changes the work, a step that runs on every lane and changes nothing, a question that keeps coming back: each is ceremony, a candidate to narrow or drop. The same kind of ask again and again points at the instruction of whoever sends it; a Lead that must reread a whole record to follow its Peer points at what a handback carries; you settling what a Lead could have points at the boundary between you. More reviews or more challenges is no gain in itself: the count that matters is what changed.
5. **Match the notebook.** A group matching a row raises its Seen and Last; a recurrence under an `applied` row means the fix was too weak, a stronger finding than a new row. An unmatched group becomes a row at `seen` when its mechanism is new, or when it shows an existing row more sharply than that row does; the same thing in different words adds nothing. Seen again on a different day, the row moves to `adopted`.
6. **Propose at most one change:** the group with the highest count and clearest class, as the smallest diff to one file (a prompt line, a skill step, an acceptance habit, a role setting), with its two dated episodes, its class, and what would show it made things worse. One change per retrospective keeps its effect attributable. Removing a rule whose episodes stopped counts as a change. Most changes belong to this project, not to every project: instructions fit a model, a kind of work and a codebase, so propose one as a rule for that role: a file the Human names in `prompts:` of `~/.piggery/config.yaml` for that role, which reaches every participant of that role whose session starts after, whatever harness it runs. Propose a change to the template's prompt (in `~/.piggery/templates/<template>/`) only when the episodes behind it would recur in any project.

Judge the system, not the agent: "the task held one directory and the work needed two" is a finding, "the Peer was careless" is not. Keep what a log says apart from what you infer from it.

## Ends in

Updated notebook rows, committed in `NOTEBOOK.md`, and the proposal as a diff for the Human in your reply, with its row naming the file under Fix lives in and what would show it worked under Check.
