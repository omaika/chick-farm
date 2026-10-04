---
name: pre-mortem
description: "Finds how a piece of work will have failed before it is given out, from failure stories written in the past tense, and turns their causes into the outcome, check and out-of-scope wording of its brief plus the decisions reserved for the Human. Use when work about to be given to a Lead or an executor has an outcome that is expensive, externally visible, or hard to reverse; not for work a revert undoes."
---

# Pre-mortem

The rule that matters most: every story is written in the past tense, and no story sees another.

You find how the work fails while changing its brief is still free, so its outcome, check and out-of-scope entries are earned rather than guessed. Say in one line why this work warrants it: expensive, leaves this machine, touches money, credentials or unrecoverable data, or rests on one untested assumption.

The mechanism is the tense. Each story starts from the brief having been carried out and the outcome having **failed**, and is written in the past tense. A question about what could go wrong returns a polite list of generic risks; one about what did go wrong returns the specific failure nobody wanted to raise. Keep every prompt in that tense.

## Procedure

1. **Fix the plan.** Write the outcome, check, budget, deadline and assumptions exactly as the brief will state them; a pre-mortem on unsettled wording returns risks about the wording.
2. **Name the failure.** A date and a failure the Human would recognise: "eight weeks from now the migration shipped and a week of orders can't be reconstructed", not "the project failed".
3. **Get two stories, three at most,** one per lens:
   - **Mechanism:** what state was wrong, which owner didn't hold it, what ordering or rollback broke.
   - **Assumption:** which stated premise turned out false, and what would have shown it early.
   - **Process** (when the work spans several tasks): where ownership overlapped, which decision nobody made, what acceptance let through.

   For most work write them yourself, one lens at a time, finishing each story before starting the next. When the code must be read to tell the story, give that reading to a worker you may spawn (a Lead in `slp`) as read-only work instead (`agent` action `spawn`): outcome "a pre-mortem report on <the plan>", any code change out of scope, and in its check the limits the stories need: one story per lens, each from a reader given only the plan, the named failure and that lens in the past tense, none seeing another's answer; and the full result committed as `notes/pre-mortem/<title>-stories.md` at the repository's root and named in its handback. How it meets those limits is its own. Read that file, then stop the worker (`agent` action `stop`).
4. **Merge, dropping nothing for being unlikely.** Keep every distinct cause; one you can't place in the system is marked unplaced, not deleted.
5. **Turn each cause into a row.** A cause the brief can't act on is a worry, not a risk.

   ```text
   R1  Failure        what had happened, past tense, one sentence
       Cause          the mechanism, assumption, or coordination gap behind it
       First signal   the earliest thing someone could observe, and where
       Mitigation     the smallest change to the plan, or none available
       Disposition    accepted | mitigated | no-go | reserved for the Human
   ```

   You decide each disposition. A `mitigated` row becomes a line of the check or a sentence in the outcome; a `no-go` row becomes an out-of-scope line; an `accepted` row whose first signal the worker can watch goes into the outcome as the point where it stops and asks. A row is reserved only when its mitigation changes the project's concept: ask the Human before giving the work out.

The output can shrink the outcome, add an out-of-scope line, or stop the work; it never enlarges the budget, and a better plan is a redesign for whoever does the work, not part of this. When similar work returns the same rows twice, add a row to `NOTEBOOK.md` at the repository's root and stop running it for that class of work.

## Ends in

The brief with its fields filled, given out as the worker's spawn task or a `send` of kind `task`; and the named failure with its rows ordered by how early their first signal appears, committed as `notes/pre-mortem/<title>.md` at the repository's root.
