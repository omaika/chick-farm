---
name: council
description: "Settles one non-trivial architecture, code, product, research, strategy, policy, or incident decision inside a lane with sealed independent reviewers you spawn, bounded verification, and one binding verdict you draft alone. Use when a decision has several defensible answers and is expensive or hard to reverse, or when whoever gave you the lane asks for one; not for a choice a cheap slice settles, and not when the whole team is a council (its template already runs this)."
---

# Council

You run the protocol and adjudicate; the reviewers do the analysis, so don't do it for them. Each position is a reviewer you spawn (`agent` action `spawn`, a role you may spawn) to read, never change, the code at the snapshot; its task, its first mail, carries the neutral brief, the case output contract and one role instruction, its name the case ID, role and round (`c3-challenger-r1`), and its report comes back as its handback. Your role's routing keeps reviewers from reaching each other. Reviewers count toward the team's concurrency limit: count the free places before you choose a tier.

```text
tier    -> the smallest sufficient tier, in one sentence
brief   -> neutral brief + case output contract + framing lint
sealed  -> spawn every Round 1 reviewer, end your turn, act on no handback until all are in
model   -> reduce the reports to the case's natural decision units
verify  -> bounded Verifiers for material factual disputes only
cross   -> at most one challenge and response per disputed unit
draft   -> you draft the verdict alone
audit   -> an Auditor, per tier
verdict -> binding verdict and the tasks it starts
```

Re-anchor on this list whenever you're unsure which step is active, across turns, until the verdict.

## Tier

`lens`: one Independent. `debate` (default): Independent + Premise Challenger. `debate-with-proof`: plus Verifiers as needed and a draft audit. `high-risk`: plus optionally one Specialist, and a mandatory audit. Say why in one sentence, like "`debate-with-proof`: the choice turns on one disputed throughput fact a Verifier can settle."

Reviewers of one model are sealed but correlated: treat their agreement as weak evidence, look hardest where they agree without independent sources, and list "single model family" under the verdict's limitations. A reviewer runs on the model its role's template sets; you don't choose it.

## 1. Neutral brief

Fill the neutral brief in [references/report-format.md](references/report-format.md): the request verbatim, a decision question that clarifies but never narrows it, facts with provenance apart from claims, constraints apart from preferences, the scope reviewers may read, and the snapshot commit. Build the case output contract from the request's natural units, asking only what comparing evidence needs; the file also holds the task texts, claim types and statuses. Work only on a task's branch is read by spawning reviewers with `cwd` set to that task's worktree.

**Framing lint.** Repair the brief until it preserves the request, implies no preferred verdict, marks unverified premises as claims, excludes no option without authority, and keeps every unit the requester expects with no filler. Ask whoever gave you the lane (`send`, kind `ask`) only when missing authority or scope would change the decision. Then start Round 1 at once.

## 2. Sealed Round 1

Spawn every reviewer in one turn with the same brief and contract and one role instruction; every reviewer's task opens and closes with the report patterns' two texts, verbatim.

- **Independent:** reason from first principles, recommend the strongest answer, expose decision-critical assumptions.
- **Premise Challenger:** test the framing and shared premises and build at least one viable counterfactual, without manufacturing disagreement.
- **Specialist:** apply only the requested domain semantics; expertise doesn't outrank stronger evidence or product authority.

Reveal no opinion, other report or reviewer's name. End your turn, and act on no handback until all of Round 1 is in. Keep every reviewer running until the verdict, since cross-examination asks it again. A silent or failed reviewer gets one `send` or one replacement (read its log first with `agent` action `tail`), and a report missing decision content one `send` asking for it. `debate` may go on with one core reviewer missing only as `DEGRADED`; `lens` and `high-risk` may not. If relevant source moved past the snapshot, stop and report the mismatch.

## 3. Decision model

Reduce the reports to the smallest model keeping every unit the verdict needs: three to five propositions for a focused decision, a row per finding, gate or obligation, a bounded timeline for an incident. Never merge, cap or drop requested findings; decompose instead. Only facts get verified, and insufficient coverage never shows a proposition false.

## 4. Verification and cross-examination

For a material factual dispute, spawn one to three Verifiers, each with one proposition verbatim, the sources and a distinct mandate: support, disconfirm, or audit coverage; never identical tasks as a vote. When the reports split on a claim a command, a test or a trace through the code can settle, a Verifier runs that check and its result settles the claim: arguments, however many agree, do not. Where evidence leaves a material disagreement, `send` the original reviewer only the disputed unit and its evidence, for a cross-examination response in a second handback.

## 5. Draft, audit, verdict

**Draft alone**, weighing the outcome and hard constraints, which premises hold, fit under realistic failure, reversibility, and whether dissent has stronger evidence. Don't vote or average. Reread the positions in reverse order of arrival and check whether that changes which you favour.

**Audit** the draft (optional in `debate`, default in `debate-with-proof`, mandatory in `high-risk`) with an Auditor whose task holds the brief, the reports by role, the model, the draft and the dissent. Resolve each material finding by revising, removing the claim, or returning it to its step.

**The verdict**, in the requester's words: the decision and why, which claims stand, required action and ownership boundaries, validation, dissent and your answer, limitations and reopen conditions, and whether the run was degraded. Commit it as `notes/council/<case-id>.md` at the repository's root, and name it in your handback.

## Stopping rules

- One sealed Round 1; one retry per reviewer; one challenge and response per disputed unit, new facts sent to verification; one audit round.
- No voting, group chat or shared room: in a shared room the most assertive model wins, not the best evidence.
- The council ends at the verdict: a task given to a Peer carries its action as goal and check, its limits as out of scope, where to start, and its decisions as context.
- Stop each reviewer (`agent` action `stop`) once you have no further question for it: an idle one still holds a place and costs money.
