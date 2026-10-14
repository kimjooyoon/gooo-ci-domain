# Verification work as a domain

## Objects and obligations

A source revision owns intent. An observed input has a consumer relation and an origin. A verification unit has an exact command, prerequisites and a resource profile. An execution origin owns its elapsed time, completion state, source tree, input reads and output digest. A plan preserves the reverse-impact test population.

The physical graph is a projection of those relations. Global input relations are factored through a shared domain node, so the representation scales with inputs plus consumers. The observation population and deadline remain explicit.

## Time model

Scope time is queue + setup + collection/planning + controlled execution + completion/transport. Already elapsed time is measured at this invocation. Rebinding/completion reserve and unit work costs use matching prior origins. The current CLI controls collection commands, package execution and child-process cancellation. GitHub scheduling and external artifact transport have separate control boundaries. Filesystem completion and operating-system scheduling can also affect observed lateness.

Under the calibrated unit costs, a plan is bounded below by its longest dependency chain and serial work divided by available workers. These are conditional forecast bounds. Future execution times remain observations. A chain whose estimated cost exceeds the budget requires a change in the work structure. Increasing parallel capacity keeps the same required population.

The source ceiling is 180,000 ms. The planner never fits a plan by extending it. An incomplete scope, absent context or timed-out context remains visible. First-seen cost discovery has a source-owned permission and the same total deadline. It yields current verification only when every required command actually completes successfully in that scope.

## Inefficient development patterns and current responses

| Pattern observed | Domain problem | Implemented response | Remaining integration |
| --- | --- | --- | --- |
| File suffix chooses `go test ./...` | Classification lacks consumer relations | Package imports, test imports, embeds and reverse closure | Compiler command adapters for the six canonical checks |
| Broad changes repeatedly trigger many observer workflows | Execution has no shared work identity or cost boundary | Exact-source start reservation and common deadline | Coordinate existing compiler workflows through one domain plan |
| Saved command contract differs from the workflow command | Timing origin and operation identity drift | Cost key derives from the executable argument definition and environment/resource profile | Ingest remote clocks only when their operation and measurement extent match |
| Historical cases pin the current registry count | Current capability and historical cohort identities overlap | Retained compiler source separates current source-selected scopes from historical denominators | Reduce duplicate fixture maintenance using source-owned capability relations |
| Timeout counted as a finished sample | Cost semantics lose unfinished work | Censored floors, missing-cost causes and explicit next action | Use additional new-source observations to assess reliability |
| Every unowned input repeats edges to every test | Representation multiplies the same relation | Shared global-input domain, preserving every consumer | Source/input provenance can establish finer consumers |
| Failure is retried before its original state is known | Execution origin is lost or ignored | Start origin persisted before commands; same source tree/profile cannot execute again through the coordinator | Export incomplete-origin diagnostics across workflow adapters |
| Observation files are committed with each source change | Measurement becomes another source change | External append-only bounded journal | Remote artifacts need their own transport cost and retention policy |

The compiler's original PR #1575 family finished with 38 successful workflows and one failed core workflow. Its race package reached the 600-second timeout. That source remains public and unmerged. These origins informed the domain work; the original workflows were not rerun.

## Measurements

- Obligation preservation: selected package identities equal the derived required population.
- Current completion: successful current command origins divided by required commands; compile-only packages are distinguishable by their input-log state.
- Forecast availability: contexts with completed usable observations divided by required execution contexts, including control reserve.
- Miss: observed command time minus its available prediction. Unavailable predictions have their own state.
- Deadline lateness: observed scope time beyond the declared ceiling. Missing results remain unresolved.
- Amplification: required consumers, global inputs and prerequisite population relative to changed inputs.

These quantities separate scope, prediction and current behavior. File count or changed line count alone cannot supply execution time.

## Next source work

Use Gooo provenance and explicit input ports to bind currently global fixture inputs to their actual consumers. Observed Go testing reads supply candidate relations. A candidate can support exclusion only after its source and collection extent establish that obligation. Then connect format, vet, race, semantic and policy producers to the same graph and clock. Independent decision models can rank source-permitted schedules with identical obligations.

The reusable Go structs are `Graph`, `Observation`, `Estimate`, `Plan`, `Execution` and `InputOrigin`. The package collector currently adapts Go-test units. Native differential and external observer commands can use the generic work graph once their command extent, input origins and completion clocks are bound. Job names alone do not identify compatible cost contexts.
