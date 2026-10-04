---
name: grilling
description: "Settles with the Human what new work should do before any lane opens: numbered rounds of questions, each with a recommended answer, until nothing about what the project does or how it behaves is assumed, and every settled answer is in CONTEXT.md at the repository's root. Use when the Human brings new work or a behavior change CONTEXT.md does not answer; not for a tiny change, a question CONTEXT.md settles, or work already settled with the Human."
---

# Grilling

The rule that matters most: facts are yours to find, what the project does is the Human's to say,
and no lane opens until the Human agrees you have understood. A lane here is any work you give out:
a Lead's lane, or an executor's task.

## What goes to the Human

Only what changes what the project does or how it behaves: who it is for, what happens in the cases
that matter, the rules its logic follows, what it will not do, and the words it is spoken of in. Stack and
architecture across lanes are yours: decide them, list them at the foot of the round under
**Assumed**, one line each, so the Human can overturn one, and do not ask; how a lane is built is its
Lead's.

**Assumed** holds only what no user or caller would notice. A line that changes what a caller sends or gets back, how
long something lasts, or what a repeat does is behavior, so it is a question: an "ok" to the whole list is no answer on
the behavior inside one of its lines, and the lanes build that line as if it were.

A fact the repository or the tools can give you is never a question: read only what settles it, and ask the rest of the round meanwhile.

Some constraints leave no trace in the repository, so ask for each the work reaches, early: a budget or deadline, a
stack or service it must use, the scale it must bear, who uses it, a contract others already depend on, and the shape
of data that already exists. Missed, each one is found only when a lane built without it has to be built again.

## Contracts callers meet

When the work has an interface others call (an API, a file format, a command), ask each of these it reaches as a
question of its own, with a scenario at its edge:

- the body of an error a caller gets;
- the code for each kind of error;
- how long a repeated request is recognized as the same one;
- what a repeat with the same key and a different body gets;
- whether the user can end a session;
- when a session ends by itself;
- how long data is kept;
- what happens to data once that time is up;
- the name of each field a caller sends or reads, one name for one thing across the whole interface.

One condition per question: a question with two gets one answer, and the other rides on it unasked. Settled, these go
into CONTEXT.md before any lane builds on them, since a test written against a contract nobody settled invents one;
what several lanes meet on then runs first, as one piece of work every other lane waits for.

## Rounds

Map the request as a tree: every decision branches into the ones that hang on it. A round asks every
decision whose prerequisites are already settled, and no other: one hanging on a question still open
belongs to a later round. Number each, give your recommended answer, and wait for the Human's answers before the next round.

```text
❓ **Q1 - <title>**: <the question, with the choices when there are some>

➡️ <your recommended answer, and why in a line>

---

❓ **Q2 - ...**

**Assumed:** <what you decided yourself, one line each>
```

Each answer reshapes the tree: ask what can be asked now.

- **Sharpen vague words.** When the Human says "account", ask whether the customer or the user is
  meant, and propose the term to keep.
- **Test with a scenario.** When a rule is stated, invent the case at its edge and ask what happens
  there.
- **Say when the words disagree** with CONTEXT.md or the code, and ask which is right.

## Writing it down

Write each answer that settles a behavior or a term into `CONTEXT.md` at the repository's root the
moment it is settled, in the shape of [references/context-format.md](references/context-format.md);
one that changes an earlier answer replaces its line. Create the file with the first settled answer,
not before. Commit it before you give out any work that rests on it: a lane in a git worktree of its
own sees only what was committed on the branch it started from.

## Read-back

Before the first lane opens, give the Human one screen to correct: the lanes you will open, each with
its outcome and its check, the contracts the lanes meet on (those above, and the module signatures you
chose) and the piece of work that settles them first, what you assumed, the defaults you will take for
them when a question comes up while they are away, and what will bring them back (a question only they
can answer, an act that cannot be undone).

Say plainly what nothing guards but you: no path waits for the Human by itself. Offer the kinds this
work reaches among access (auth, login, session, passwords, secrets, credentials, tokens), money
(payments, billing) and what ships (CI workflows, Docker, `.env`, infra, deploy, terraform, k8s, helm);
for each they keep, every brief that reaches those paths says to stop and ask before changing them, and
you bring the change to them before it is merged. Name the risky kinds of change this work reaches
(migrations, schemas, SQL), and ask for a command that rehearses one, such as a migration run twice on a
copy, where they have one; it goes into the check of each brief that reaches it.

A correction is a settled answer like any other. In their words, what they want to be woken for goes
under `## When to wake the Human` in `NOTEBOOK.md` at the repository's root, and each default they
confirmed under `## Confirmed defaults`, one line each; edit the file, keeping its other sections and
rows, and commit it with `CONTEXT.md`.

## Ends in

Every branch visited, nothing about the concept silently assumed, and the Human's confirmation that
you have understood. If the Human says to start before that, start, and name what is still open in
out of scope or as where its Lead must ask.
