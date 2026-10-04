# NOTEBOOK.md

The supervisor's notebook: the coordination patterns this project keeps producing, one row each, and
where each one's fix lives. It lives at the repository's root and is committed. Read it at the start
of a session and match what you see against it before acting.

## Where things go

| You have | It goes to |
|---|---|
| An event, a quote, a time or a SHA | nowhere: piggery keeps events (`piggery --admin log`) and mail (`piggery --admin mail`) |
| A ruling on work in flight | a `send` to the worker doing it |
| A rule for code in this repository | a task asking a worker to put it in `AGENTS.md` |
| What the project does or how it behaves, as the Human settled it | `CONTEXT.md` beside this file |
| A pattern, new or seen again | a row under Patterns |
| What the Human wants to be woken for, in their words | a line under When to wake the Human |
| A default the Human confirmed at read-back | a line under Confirmed defaults |
| The Human correcting what you told them | a dated line under Corrections |
| A change to a prompt, a template, a role's rules or a harness profile | a diff for the Human |

## Working method

- A row is a mechanism, not an episode: "a brief that states the expected answer gets it back
  unchecked", not "the parser brief on 09-13".
- A first occurrence goes in at `seen` when it is genuinely new, or when it shows something you
  already have a row for more sharply than that row does; the same thing again in different words
  does not. The second sighting, on a different day, moves the row to `adopted`, naming one place
  its fix lives; `applied` when that fix exists; `verified` when its Check has held.
- Prefer a change to authority, information or integration over one more rule.
- When it gets long, fold rows into the pattern they are all instances of rather than dropping the
  oldest. A `verified` row whose fix has held for weeks has done its work and can go.

```md
# Supervisor notebook

## When to wake the Human

## Confirmed defaults

## Corrections

## Patterns

| ID | Pattern | State | Seen | Last | Fix lives in | Check |
|---|---|---|---|---|---|---|
```
