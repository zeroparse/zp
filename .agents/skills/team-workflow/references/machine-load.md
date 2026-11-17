# Machine Load and the Run Queue

Many lanes share one machine, and the CPU is the bottleneck. Unthrottled parallel test runs
once drove the load to 180 on 16 cores and slowed every gate. Timed measurements also need a
quiet machine. So runs fall into three classes.

| Run | How |
|:--|:--|
| Docs, spec, site and skill checks (`cd site && go test -count=1 ./...`, the site build and check), `go vet`, deterministic count or charge probes that measure no time | Directly, throttled: `nice -n 15 env GOMAXPROCS=4 ...`. Never queue them. |
| Targeted package tests while developing | Throttled through the test slots: `.agents/scripts/testslot.sh nice -n 15 env GOMAXPROCS=4 go test -p 2 -run '<tests>' -short <pkgs>` |
| Whole packages of the large test suites (engine, codegen and complexity tests), the root suite, `-tags zp_long`, `go generate ./internal/codegen/... ./bench`, `hack/test-alpha-parsers.sh`, `hack/test-reference-tags.sh`, benchmarks and any timed run | Only through the queue: `.agents/scripts/benchlock.sh <tag> zsh -f -c 'cd <worktree> && ...'` |

Heavy test runs, including the root `go test ./...`, go only through the queue. Docs and site
checks and count probes run directly, throttled.

## The queue

- `.agents/scripts/benchlock.sh <tag> <command...>` waits for the shared heavy-run lock and
  then runs the command in its own process group. Its record is `runs/<tag>.<pid>`; waiters are
  served in (`start=`, `seq=`) order. A line in `benchlock-stalls.log` is a queue bug: report it.
- `.agents/scripts/queue-status.sh` shows the queue and exits 1 when it is stuck (jobs queued,
  none running) or a stall was logged. Inspect at once when it does.
- `.agents/scripts/run-gate.sh LEVEL...` picks the run class of a `hack/gate.sh` level for you.
- Use `zsh -f` for queued commands. A login shell profile can reset PATH and drop `node`.
  The TypeScript parity tests then fail with "Node is required", which is the environment,
  not the change.
- Stop your own job with `.agents/scripts/benchstop.sh <tag>`. It signals only the recorded group. Never kill
  a process you did not start. Before any manual kill, confirm in the same command that the
  PID is still yours (`ps -o command= -p <pid>`), because PIDs are reused.
- Priority: landing gates and checks that unblock another lane go before measurement
  probes and exploratory runs. The leader may move a probe's record later by editing its
  `start=`; do not move it back. The integrator sets the `start=` of a train record ahead of
  every non-train job; only train records are moved forward. Keep probe jobs few and
  batched.
- Never idle while your job waits. Continue the next independent step of your lane in a
  separate worktree, never in the worktree a queued or running job tests. If your lane has no
  independent step, tell the leader in one line.

## Measurements

- Follow the [performance skill](../../performance/SKILL.md) for the A/A landing rule, the
  run records and the measurement method.

## Caches and disk

Tests that build generated programs make a unique program per test, so their build cache
entries are never reused. A shared Go build cache once grew past 1 TB and filled the disk.
Every test package that builds generated programs calls `codegentest.SetGoCache` in its
`TestMain`. If the disk fills, find the producer. Then clean the caches through the queue:
build and test caches may always be deleted.
