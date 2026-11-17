# Landing Train

Full gates are expensive: the long-tier root suite, generation and the alpha-parsers check
together take hours. They run only on train tips, never once per lane.

## Queue file

`$ZP_AGENT_STATE/landing-train.md` is append-only for lanes. It starts with this line:

```text
Format: READY <branch> <sha> base <sha> | <scope, Refs #N> | <checks run, each with its exit status> | <review verdict file>; the leader's go is a separate LEADER go line
```

- **READY**: a lane runs `.agents/scripts/ready.sh "<scope, Refs #N>" "<checks, each with its
  exit status>" ["<review verdict file>"]` after pushing (it appends the line with the SHA and
  base), then messages the integrator. A
  new READY line for the same work says which line it supersedes.
- **LEADER go**: the leader appends `LEADER go <branch> <sha> (<why: review verdict, perf
  result>)`, and names the train when ordering matters. A candidate enters a train only after
  its acceptance conditions are met: review, perf and the leader's go.
- **Other lines**: `WITHDRAWN`, `HOLD`, `NOTE` and `ANNOUNCE` (for example a rename that
  every lane must rebase onto). The integrator writes `TRAIN-<n>` and `LANDED train-<n>
  <date>: main <old>..<new> (<commits, issues>)`.
- The leader moves finished history (LANDED, WITHDRAWN and superseded lines) into
  `landing-train-archive.md`. Do it in one quick read-modify-write, and re-read the file
  right before writing so that no concurrent append is lost. Keep every current READY line,
  every LEADER line and every open note.

## Lane checks before READY

- `.agents/scripts/run-gate.sh docs` or `code` (the levels of `hack/gate.sh`; see
  [machine load](machine-load.md) for the run classes).
- Lanes do not run the `engine` or `full` levels (generation, alpha-parsers); the train gate does. When the generator changed, diff the
  `zp gen` output of the affected grammars against main.
- A change to evaluation charges, emitted code or generated output runs the golden and
  mutant tests that pin them, even if they sit in the long tier. Run them targeted:
  `-tags zp_long -run 'TermCharges|OutputManifest|Mutant|DetectsMutation|Charges'` in the
  affected packages. Updated goldens land in the commit that changed them, with the reason in
  its body.
- A new or changed test that asserts charges, budgets or limits runs under
  `hack/test-reference-tags.sh`. Budget controls compile out in the reference build, so skip
  only the charge assertion there and keep the behavior checks. Missing this once failed a
  train after a two-hour gate.
- A change that removes or renames an exported name that alpha-parsers imports runs
  `go vet ./...` in a private alpha-parsers clone pointed at the branch. It lists the
  hand-written alpha files to edit in its READY line. Hand-written alpha-parsers edits need
  the owner's OK for each landing.
- Lanes never use the integrator's alpha-parsers checkout: `.agents/scripts/alpha-check.sh -c`
  works on a private clone in `.scratch/alpha`.
- Profile structural and feature changes before READY (see the
  [performance skill](../../performance/SKILL.md)).

## The integrator

- Stacks ready candidates with a go on current main, in the order the leader set:
  `.agents/scripts/train.sh stack <name> <rev>...`. Runs one full gate on the tip through the
  queue: `train.sh gate <name>` (levels `engine full` of `hack/gate.sh`, one summary line per
  step).
- On a pass: `train.sh land <name>` pushes main without force and appends the LANDED line; then regenerate and push alpha-parsers when the
  protocol or generated output changed (generated files only, never force), run the backlog
  close step, and delete the landed remote branches.
- On a failure: bisect with targeted checks. Send the failure to the owning lane, or fold
  a small fix into its owning commit with the lane's agreement.
- Trains run about twice a day and take every READY item with a go. After every train, list
  the READY items it left out, and give each one a rebase owner in the same step. Items left
  out of several trains fall far behind and rot.

## Fast lane

A candidate that changes only docs, spec, site or skills, or only moves or edits test files
(no non-test Go code), takes the `docs` level of `hack/gate.sh` (`code` when test files changed), through
`.agents/scripts/run-gate.sh`. It skips zp_long, generation and alpha. The
integrator may land it between trains. A train then rebases onto it
without rerunning its gate when the intervening commits are fast-lane only. Before queuing a
full gate, check its scope against what the batch actually touches.
