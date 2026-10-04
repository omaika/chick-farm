# CONTEXT.md

What the project does and how it behaves, as settled with the Human. Every agent that works on the
project reads it before acting on a behavior or a term; it is not a design document. It lives at the
repository's root and is committed.

```md
# <project>

## Purpose
<who it is for and what it does for them, in two or three lines>

## Terms
- **<term>**: <what it means here>. Not: <the words not to use for it>.

## Behaviors
- <a rule the project follows, in the present tense>. Settled by: <the edge case that decided it>.

## Contracts
- <an interface others call>: <what a caller sends and gets back; errors and their codes; how long a
  repeat counts as the same request; sessions; how long data is kept; field names>.

## Not doing
- <what the project will not do, and why when the why is not obvious>

## Open
- <a question still open, and who answers it>
```

- One fact per line, in the present tense. A changed answer replaces its line: history is in git.
- A term has one meaning across the file, and a field one name across every contract.
- No stack, module or file choices: they are the builders', and change without the Human.
- Leave a section out until it has a line.
