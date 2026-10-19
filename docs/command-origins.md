# Source-bound command work

`work` adapts a real command to the same time domain as the package planner.
It records one command's outcome. The caller owns the required command
population and its prerequisites.

```sh
gooo-ci-domain work --root /path/to/repository \
  --head EXACT_COMMIT --unit urn:project:ci:vet \
  --execution-id "$CI_RUN_ID/$CI_RUN_ATTEMPT" \
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

A first-seen command can run within the remaining budget and records an
unavailable forecast. A later execution can use its completed cost observations.
`--execution-id` identifies the current run and attempt, separate from the
reusable cost key. The same source command can run again in a new execution
after restoring a historical cost journal. A repeat within one execution is
refused, including when its earlier start was reserved but not completed.
Omitting the flag retains the original single-origin behavior. The ID is
recorded in the command receipt and journal; it does not alter the cost key.
A timeout is censored and cannot become a successful completed cost sample.

## Common clock

All commands receive the same caller-declared scope start and explicit request
ceiling. Setup, earlier stages and any waiting after that start consume this
scope. Each command has only the remaining time;
the next command cannot reset its request clock. Process cancellation
covers the command's process group, including ordinary child compilers.

This repository's CI starts admitted work in its first runner step, before
checkout or setup. Its artifact retains the original GitHub API response in
`workflow-run.json` and the admitted start in `work-started-at.txt`. Run ID and
attempt must match the current job, and the observed start cannot precede run
creation. The 180-second work budget and race-test command are unchanged. A
caller that owns an end-to-end deadline can instead supply run creation as its
scope start; in that case initial queue time consumes the declared budget too.

The original [PR #5 run](https://github.com/kimjooyoon/gooo-ci-domain/actions/runs/39087360703)
used run creation as the scope start. It waited 327 seconds for a runner and
failed 12 seconds after job start, before building or executing tests. This
failure is retained. The revised scope makes the controlled work budget explicit
while keeping initial waiting in the original remote elapsed-time record.

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

The CLI can return a nonzero status with a JSON receipt. Receipt fields describe
the observed command outcome; the CLI status also covers record persistence and
receipt output. Preserve the original CLI status and diagnostic log alongside
the receipt. A retained `COMMAND_COMPLETED` result can accompany a failure to
store its final observation. Use the combined result to decide the next action;
receipt presence alone provides no permission to repeat the command.

An outcome observation can also record failure to start a process, with
`ORIGIN_UNAVAILABLE` and `Completed=false`. Errors returned before an outcome
observation exists keep the existing empty-stdout path. Normal admission refusals
still emit their existing JSON and nonzero status. Failed receipt delivery can
leave partial stdout; preserve the error and treat those bytes as incomplete.

Non-successful domain results also emit one diagnostic line to stderr. It carries
the existing decision, admission, next operation, predicted cost (or `unknown`),
scope/deadline/unit milliseconds, completion flag and command exit code. These
values come from the same receipt; the display adds no policy decision or clock.
For example, `OVER_BUDGET` with `REFINE_REQUIRED_WORK` identifies an admission
refusal before a command observation. A started command may instead produce
`VERIFY_FAILED` or `DEADLINE_EXCEEDED`. The complete receipt retains that distinction
and its origins. The diagnostic omits command arguments, source paths and output.
Successful commands keep their existing log stream without an added summary.
