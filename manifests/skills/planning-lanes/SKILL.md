---
name: planning-lanes
description: "Decides how a high-risk lane is built before any task starts: whether it is high-risk at all, the final contract written first, where the work splits into tasks, which design choices to settle, and how to get back if it fails, kept as the lane's plan page. Use when a lane touches auth, money, data loss, migrations, concurrency or a shared contract. Not for a lane without such risk, however many tasks it splits into."
---

# Planning lanes

The rule that matters most: write the final contract first, split only where you can name the reason, and plan no lane you cannot get back out of.

## Is it high-risk?

A lane is high-risk when it materially changes:

- authentication, authorization, privacy, audit or secret handling;
- data: loss, an irreversible migration, deletion, retention, replay or recovery;
- money, credentials, user-visible delivery, or an external side effect that cannot safely run twice;
- a current contract replaced in coordination;
- runtime ownership, concurrency, lifecycle or ordering;
- a proof that protects a security, data, contract or external claim;
- compatibility: a fallback, shim, dual read or write, legacy parser or version branch.

A label alone does not make a lane high-risk; material impact does. A normal lane needs no plan page: its brief and its check are the plan.

## Find out first

The plan rests on what the code shows, not on what you expect it to show, so a first read of the code answers before the plan is written: a Peer you give that question as its task, or your own reading when it is small. Its findings fill Known; what it could not check fills Assumed, each with the task that checks it first. A finding that contradicts your brief goes to whoever gave you the lane (`send`, kind `ask`) before a task rests on it; a question its answer opens goes back to the same Peer with `send`, not into your own reading of the code.

## Split for agents

Split the way the work divides, not by a count: pieces that do not call each other run as parallel tasks, and the one that wires them waits for both.

- Split only for a reason you can name: work whose paths do not meet and can run in parallel, a mechanical fan-out too big for one sitting, separately accepted deliverables, or shipped production state that needs a staged change.
- Never split by layer, to show progress, or into phases that keep a half-built state compiling: one writer changes a contract with all its callers and tests.
- Each task hands back green: its checks pass on its own. A task may leave the build red for the next only when the plan says so, names the task that turns it green, and nothing is merged in between.
- Parallel tasks resting on the same unchecked assumption about the environment or a contract: give one out first and the rest only after its handback, so a wrong assumption costs one task, not all of them.
- For a lane heavy on one contract, a task of its own may write the acceptance tests from the settled contract alone, before or beside the tasks that build it, so the tests are not fitted to the code.
- A compatibility layer is legitimate only for a named shipped consumer: a published API, persisted production data, an independently deployed service or client. Record the consumer and when the layer goes; everything else changes in place.

## Settle design first

Settle every choice that changes ownership, public behavior, safety, compatibility or data, or is expensive to reverse, before a task that depends on it starts: the options weighed and why, never files, symbols or control flow. A choice with several defensible answers goes to the `council` skill; one only the Human can make goes up to whoever gave you the lane (`send`, kind `ask`), your default with it.

## The plan page

Keep it pinned on the team board (`send` to `board`; `op: "replace"` with `target` the pin's #N to change it), from the template in [references/plan-page.md](references/plan-page.md): outcome, final contract, one row per task with why it is separate, what is known and what is assumed, intermediate states, decisions, the end check, and getting back. It holds the present only, under 80 lines, replacing lines rather than adding them, so a successor can resume the lane from it; a handback that changes what you know changes the page in the same turn.

Nobody approves a plan before its tasks start. Where a wrong plan would cost the rework of several tasks, put the plan page to the `council` skill first.

Getting back is not optional for a lane that migrates data, writes outside the repository, or makes a call nobody can take back: a plan that says how to reach the outcome but not how to get out of it is missing the half needed under pressure. A lane that leaves nothing behind says so in one line.

## Ends in

The plan pinned on the board, then the tasks it names given out: each as a Peer's spawn task or a `send` of kind `task` with `op: "assign"`, each saying what it waits for, and the paths it holds when it runs beside others, only where they meet no other task's: as narrow as you know them, a folder where you do not. Tasks that run at the same time on one repository get their own branch and worktree, as your role says.
