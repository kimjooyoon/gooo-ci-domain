# One work allowance across runner gaps

The compiler's [run 40139878025](https://github.com/kimjooyoon/meta-ontology-go/actions/runs/40139878025)
passed its six canonical checks. Its proof job then failed during initialization:
the 50-minute shared clock included time waiting for another runner. At proof
binding, 54m54s had passed since preparation; all six checks were already green.

`runner-clock` observes the **union of runner intervals** for the exact source,
run and attempt. Overlapping intervals count once. Sequential jobs continue to
spend one allowance. A running job's polling, setup and artifact transport count.
Only periods with no observed job running are separated from charged work.
This is a conservative work-time boundary, not CPU usage or a critical-path
forecast. It does not reduce test populations or certify check outcomes.

```sh
gooo-ci-domain runner-clock --run-json run.json --jobs-json attempt-jobs.json \
  --head EXACT_HEAD --run-id RUN --attempt ATTEMPT --budget-ms 3000000 \
  --job 'CI proof bundle' --producer 'Work domain preparation'
```

The caller retains original GitHub API responses. The jobs response must contain
the complete paginated current attempt: `{ "total_count": N, "jobs": [...] }`.
Input hashes, job IDs, observation time, original workflow creation, wall time,
active time, runner-free time and remaining allowance accompany the decision.
Charged jobs require a positive runner identity. Unassigned job IDs are retained
separately. GitHub can populate `started_at` while a job is still queued; that
timestamp alone does not establish runner work. Cancelled/skipped jobs with no
runner and no steps also retain their timestamps without spending work time.
Missing assignment for a running/completed executed job, contradictory queued
assignment, or cancelled work with steps but no runner refuses observation.
Missing intervals, foreign identities, duplicate IDs, future/reversed timestamps,
truncated populations, an absent current job or an unsuccessful producer refuse
admission. The current job must be in progress. Inputs are bounded to 4 MiB each
and 4,096 jobs. GitHub's API is the observation authority; missing API data is not
reconstructed from local guesses or earlier attempts.

The source activity `AdmitRunnerClock` in `policy.gooo` owns admission. The Go
adapter observes physical timestamps and computes interval union. Its generated
activity and original generation receipt are linked in `origins`.

`effective_started_at = observed_at - active_elapsed_ms` translates that observed
allowance into the existing command deadline. It is a derived coordinate, not a
replacement for any original timestamp. While the current job keeps running,
the union grows continuously with wall time. Every command in that job must keep
this same derived start and original budget. A later job observes the complete
attempt again, including all previous stages; it cannot obtain a fresh allowance.
Runner gaps before admission are therefore excluded without pausing any child
command, resetting stages or increasing the shared ceiling. Per-job timeouts
remain independently enforced.

## Recorded counterexample

The fixture in `testdata/runner-clock/1786-at-proof-bind.json` is a retrospective
projection of original attempt timestamps at 23:46:32Z. It removes the not-yet
started reporter and unobserves the proof job's later completion. It is an input
fixture, not fresh CI evidence. At that boundary:

| Observation | Milliseconds | Time |
| --- | ---: | --- |
| Workflow wall elapsed | 3,690,000 | 61m30s |
| Runner interval union | 2,334,000 | 38m54s |
| Runner-free gaps | 1,356,000 | 22m36s |
| Remaining shared allowance | 666,000 | 11m06s |

Separate cases reject sequential 60-minute work under the unchanged 50-minute
allowance and reject equality with the ceiling. A discrete occupied-second
oracle checks 256 interval populations independently of the interval-merging
algorithm. These results test accounting. Successful remote proof generation
still requires an exact-head workflow run after integration.

The first integration run, 40146229341, was cancelled before runner assignment
after observing a populated `started_at` on its queued preparation job. Its
original cancelled/unassigned job projection is retained in a second fixture.
The previous interval reader incorrectly charged its 137 seconds as work. This
counterexample is checked alongside queued timestamps and unknown assignments.
