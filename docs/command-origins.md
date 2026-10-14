# Source-bound command work

`work` adapts a real command to the same time domain as the package planner.
It records one command's completion. The caller owns the required command
population and its prerequisites.

```sh
gooo-ci-domain work --root /path/to/repository \
  --head EXACT_COMMIT --unit urn:project:ci:vet \
  --started-at "$CI_SCOPE_STARTED_AT" --budget-ms 180000 \
  --journal /outside/checkout/command-origins.jsonl \
  -- go vet ./...
```

The command's original argument vector is preserved. Standard output and error
are streamed to the caller's error stream; the JSON receipt uses standard
output. The complete output has a digest and byte count, with no growing local
capture buffer.

## Origins and predictions

The source seal contains the Git head and tree, policy digest, argument vector,
resolved executable and its byte digest, environment/resource profile, canonical
working directory digest and an optional workspace origin file digest. Tracked
active inputs must match the declared head before execution. The caller's
`GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE` remain active. Untracked command
outputs have a separate scope.

`--workspace-origin` retains file bytes by hash. Its contents are not interpreted
as an authorization. Materialization and proof producers keep that responsibility.
The source check cannot establish all external files, services or subprocess
environment inputs. A caller must supply their contracts before using an origin
to exclude other required work.

Costs are reusable across functional source changes only when the unit ID,
arguments, executable bytes, environment/resource profile and canonical working
directory match. A different runner checkout path creates a different context.
This conservative identity currently reduces cross-host reuse. Predictions are
empirical rather than proportional to changed lines.

A first-seen command can run once within the remaining budget and records an
unavailable forecast. A later source can use its completed cost observations.
An atomic start reservation prevents two callers from running the same source
tree and command context. A timeout is censored, and cannot become a successful
completed cost sample.

## Common clock

All commands receive the original workflow creation time. Queue, setup and
earlier stages consume this scope. Each command has only the remaining time;
the next command cannot receive another three minutes. Process cancellation
covers the command's process group, including ordinary child compilers.

GitHub queue scheduling, action steps and artifact transport remain external
boundaries. Job timeouts bound active jobs; the runner's clock bounds admitted
commands. OS scheduling, filesystem persistence or a blocking caller output
transport can produce observed lateness. Receipts expose elapsed scope time.

## Outcomes

Gooo owns admission and outcome classification. `COMMAND_COMPLETED` means this
exact command exited successfully and its output origin was available.
`VERIFY_FAILED` requests source repair; `DEADLINE_EXCEEDED` preserves unfinished
work and calls for recalibration. A command receipt does not authorize a compiler
release; that requires its complete current canonical check population and proof.
