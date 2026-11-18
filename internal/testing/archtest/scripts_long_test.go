//go:build zp_long

package archtest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/testing/mutanttest"
)

// scriptsDeps are what the tests of the queue scripts depend on: the scripts and the test package (the harness
// helpers live in its other files). The gate skips them when none changed since its base (see mutanttest).
var scriptsDeps = []string{".agents/scripts", "internal/testing/archtest"}

// The queue scripts' races and signal handling, run against temporary state directories. They
// take minutes, so they belong to the long tier.

func scriptPath(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, ".agents", "scripts", name)
}

func procStart(t *testing.T, pid int) string {
	t.Helper()
	out := mustRun(t, ".", os.Environ(), "ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	return strings.TrimSpace(out)
}

// rendezvousTools puts a find and a cat first on the PATH that wait until two claimers have called them
// with the same arguments (or for 1.5 seconds): the find that judges a mutex stale by its age, and the
// cat that reads the pid of a slot. The scripts judge a holder dead from what they read and act on it
// afterwards, so two claimers that are not serialized both judge the same dead holder before either
// acts, and each then takes the slot. Claimers that are serialized wait for the full time, one at a time.
func rendezvousTools(t *testing.T, env []string, dir string) []string {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"find", "cat"} {
		real, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not installed", tool)
		}
		write(t, filepath.Join(bin, tool), `#!/bin/sh
case "$*" in *-maxdepth*|*/pid)
  d=`+dir+`/$(echo "$*" | tr -c 'a-z0-9' _)
  mkdir -p "$d"; : > "$d/$$"; i=0
  while [ "$(ls "$d" | wc -l)" -lt 2 ] && [ "$i" -lt 30 ]; do sleep 0.05; i=$((i + 1)); done ;;
esac
exec `+real+` "$@"
`, 0o755)
	}
	return append(env, "PATH="+bin+":"+os.Getenv("PATH"))
}

// A dead mutex and dead slots left by killed holders, with many waiters: reclaiming must be one
// step, so no more than TESTSLOT_N commands run at once.
func TestTestslotReclaimDoesNotAdmitExtraHolders(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	state := t.TempDir()
	env := append(scriptEnv(t, state), "TESTSLOT_N=2")
	slots := filepath.Join(state, "slots")
	write(t, filepath.Join(slots, ".mutex", "owner"), "99999 Mon Jan  1 00:00:00 2001\n", 0o644)
	for _, i := range []string{"1", "2"} {
		write(t, filepath.Join(slots, i, "pid"), "99998\n", 0o644)
	}
	run := t.TempDir()
	work := filepath.Join(run, "work.sh")
	write(t, work, `#!/usr/bin/env bash
echo $$ > "$1/live.$$"
sleep 0.4
ls "$1" | grep -c '^live\.' >> "$1/counts"
rm -f "$1/live.$$"
`, 0o755)
	var cmds []*exec.Cmd
	for i := 0; i < 8; i++ {
		cmd := exec.Command("bash", scriptPath(t, "testslot.sh"), work, run)
		cmd.Env = env
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := os.ReadFile(filepath.Join(run, "counts"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(string(counts))
	if len(lines) != 8 {
		t.Fatalf("%d of 8 commands ran: %q", len(lines), lines)
	}
	for _, l := range lines {
		if c, _ := strconv.Atoi(l); c > 2 {
			t.Errorf("%s commands ran at once with 2 slots: %q", l, lines)
		}
	}
}

// Two claimers that judge the same dead mutex and slot together must not both take the slot: the claim
// is one step, serialized between the scripts.
func TestTestslotClaimIsSerialized(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for round := 0; round < 3; round++ {
		state := t.TempDir()
		rdv := t.TempDir()
		env := rendezvousTools(t, append(scriptEnv(t, state), "TESTSLOT_N=1"), rdv)
		slots := filepath.Join(state, "slots")
		// A mutex left without an owner is judged stale by its age.
		mutex := filepath.Join(slots, ".mutex")
		if err := os.MkdirAll(mutex, 0o755); err != nil {
			t.Fatal(err)
		}
		past := time.Now().Add(-3 * time.Minute)
		if err := os.Chtimes(mutex, past, past); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(slots, "1", "pid"), "99998\n", 0o644)
		run := t.TempDir()
		work := filepath.Join(run, "work.sh")
		write(t, work, `#!/usr/bin/env bash
echo $$ > "$1/live.$$"
sleep 1
ls "$1" | grep -c '^live\.' >> "$1/counts"
rm -f "$1/live.$$"
`, 0o755)
		var cmds []*exec.Cmd
		for i := 0; i < 2; i++ {
			cmd := exec.Command("bash", scriptPath(t, "testslot.sh"), work, run)
			cmd.Env = env
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			cmds = append(cmds, cmd)
		}
		for _, cmd := range cmds {
			if err := cmd.Wait(); err != nil {
				t.Fatal(err)
			}
		}
		counts, _ := os.ReadFile(filepath.Join(run, "counts"))
		lines := strings.Fields(string(counts))
		if len(lines) != 2 {
			t.Fatalf("round %d: %d of 2 commands ran: %q", round, len(lines), lines)
		}
		for _, l := range lines {
			if c, _ := strconv.Atoi(l); c > 1 {
				t.Errorf("round %d: %s commands ran at once with 1 slot: %q", round, l, lines)
			}
		}
	}
}

// stagedTools puts a find and a cat first on the PATH that stage two claimers, a and b, through the
// race that only the flock closes. Both judge the dead .mutex stale; a acts on it and takes the
// mutex; b, delayed after its check, removes a's live mutex and takes it too, so both are inside.
// a reads slot 1 (dead holder) and waits until b has read it as well; b then waits until a has
// claimed slot 1 and acts on its stale read: it removes a's slot and claims it. Each wait gives up
// after three seconds, so claimers that are serialized pass the stage one at a time.
func stagedTools(t *testing.T, env []string, dir, role string) []string {
	t.Helper()
	bin := t.TempDir()
	wait := `i=0; until COND || [ "$i" -ge 60 ]; do sleep 0.05; i=$((i + 1)); done`
	for _, tool := range []string{"find", "cat"} {
		real, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not installed", tool)
		}
		var body string
		switch role + tool {
		case "afind": // a judges the mutex only after b has judged it.
			body = `*.mutex\ -maxdepth*) : > ` + dir + `/a.mutex; ` + strings.Replace(wait, "COND", `[ -e `+dir+`/b.mutex ]`, 1) + ` ;;`
		case "bfind": // b judges the mutex, then acts only once a holds it.
			body = `*.mutex\ -maxdepth*) out=$(` + real + ` "$@"); : > ` + dir + `/b.mutex; ` +
				strings.Replace(wait, "COND", `[ -s "$1/owner" ]`, 1) + `; [ -z "$out" ] || echo "$out"; exit 0 ;;`
		case "acat": // a reads slot 1, then acts only once b has read it too.
			body = `*/1/pid) out=$(` + real + ` "$@"); : > ` + dir + `/a.slot; ` +
				strings.Replace(wait, "COND", `[ -e `+dir+`/b.slot ]`, 1) + `; echo "$out"; exit 0 ;;`
		case "bcat": // b reads slot 1, then acts only once a has claimed it.
			body = `*/1/pid) out=$(` + real + ` "$@"); : > ` + dir + `/b.slot; ` +
				strings.Replace(wait, "COND", `[ -s "$1" ] && [ "$(`+real+` "$1")" != 99998 ]`, 1) + `; echo "$out"; exit 0 ;;`
		}
		write(t, filepath.Join(bin, tool), "#!/bin/sh\ncase \"$*\" in\n"+body+"\nesac\nexec "+real+" \"$@\"\n", 0o755)
	}
	return append(env, "PATH="+bin+":"+os.Getenv("PATH"))
}

// Two claimers that judge the same dead mutex stale must not both get inside it and both take one
// slot. The order is forced (see stagedTools), so a script without the flock that serializes the
// claim runs two commands in its one slot every time.
func TestTestslotStaleMutexRaceNeedsTheFlock(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	state := t.TempDir()
	stage := t.TempDir()
	slots := filepath.Join(state, "slots")
	mutex := filepath.Join(slots, ".mutex")
	if err := os.MkdirAll(mutex, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(mutex, past, past); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(slots, "1", "pid"), "99998\n", 0o644)
	run := t.TempDir()
	work := filepath.Join(run, "work.sh")
	write(t, work, `#!/usr/bin/env bash
echo $$ > "$1/live.$$"
sleep 1
ls "$1" | grep -c '^live\.' >> "$1/counts"
rm -f "$1/live.$$"
`, 0o755)
	var cmds []*exec.Cmd
	for _, role := range []string{"a", "b"} {
		cmd := exec.Command("bash", scriptPath(t, "testslot.sh"), work, run)
		cmd.Env = stagedTools(t, append(scriptEnv(t, state), "TESTSLOT_N=1"), stage, role)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	counts, _ := os.ReadFile(filepath.Join(run, "counts"))
	lines := strings.Fields(string(counts))
	if len(lines) != 2 {
		t.Fatalf("%d of 2 commands ran: %q", len(lines), lines)
	}
	for _, l := range lines {
		if c, _ := strconv.Atoi(l); c > 1 {
			t.Errorf("%s commands ran at once with 1 slot: %q", l, lines)
		}
	}
}

// blockWait is how long a claim may take before it counts as blocked: 4 seconds when the claim
// should block, so that the test does not wait long for a claim that never comes, and 60 seconds
// when it should go through, so that a claim slowed by a heavily loaded machine is not taken for
// a blocked one.
func blockWait(blocked bool) time.Duration {
	if blocked {
		return 4 * time.Second
	}
	return 60 * time.Second
}

// A live pid with another start time is a reused pid, not the holder; the same start time is.
func TestTestslotReusedPid(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	live := os.Getpid()
	pid := strconv.Itoa(live)
	for _, c := range []struct {
		name, start string
		blocked     bool
	}{
		{"reused", pid + " Mon Jan  1 00:00:00 2001", false},
		{"holder", pid + " " + procStart(t, live), true},
	} {
		for _, where := range []string{"slot", "mutex"} {
			state := t.TempDir()
			env := append(scriptEnv(t, state), "TESTSLOT_N=1")
			slots := filepath.Join(state, "slots")
			if where == "slot" {
				write(t, filepath.Join(slots, "1", "pid"), pid+"\n", 0o644)
				write(t, filepath.Join(slots, "1.start"), c.start+"\n", 0o644)
			} else {
				write(t, filepath.Join(slots, ".mutex", "owner"), c.start+"\n", 0o644)
			}
			ctx, cancel := context.WithTimeout(context.Background(), blockWait(c.blocked))
			cmd := exec.CommandContext(ctx, "bash", scriptPath(t, "testslot.sh"), "true")
			cmd.Env = env
			err := cmd.Run()
			timedOut := ctx.Err() != nil
			cancel()
			if c.blocked != timedOut {
				t.Errorf("%s %s: blocked=%v, want %v (err %v)", c.name, where, timedOut, c.blocked, err)
			}
		}
	}
}

// lingering lists the processes whose command line holds the marker: the command, and copies of
// the script, which may still be on their way to start it or only leaving.
func lingering(marker string) string {
	out, _ := exec.Command("pgrep", "-f", "sleep "+marker).Output()
	return strings.TrimSpace(string(out))
}

// commandPid is the pid that the command wrote to pidFile, if it started, and whether that
// process is still alive and still the command: its command line holds the marker, which a reused
// pid's does not.
func commandPid(pidFile, marker string) (pid string, alive bool) {
	b, _ := os.ReadFile(pidFile)
	pid = strings.TrimSpace(string(b))
	if pid == "" {
		return "", false
	}
	out, err := exec.Command("ps", "-o", "args=", "-p", pid).Output()
	return pid, err == nil && strings.Contains(string(out), marker)
}

// commandRunning reports whether a process runs exactly "sleep marker": the command itself, not a
// copy of the script whose command line holds it.
func commandRunning(marker string) bool {
	out, _ := exec.Command("ps", "-axo", "args=").Output()
	for _, l := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(l) == "sleep "+marker {
			return true
		}
	}
	return false
}

// startLatency is how long the script takes to start a command, so that the TERMs of the test are
// spread over the window in which it starts and not over a window that the script has passed or not
// reached: a script that takes 400 ms to start under load is not startled by a TERM after 200 ms.
func startLatency(t *testing.T, name string, args []string) time.Duration {
	t.Helper()
	var worst time.Duration
	for k := 0; k < 3; k++ {
		marker := fmt.Sprintf("%d.%03d", 30+k, len(name))
		cmd := exec.Command("bash", append(append([]string{}, args...), "sleep", marker)...)
		cmd.Env = scriptEnv(t, t.TempDir())
		begin := time.Now()
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		for !commandRunning(marker) && time.Since(begin) < 30*time.Second {
			time.Sleep(2 * time.Millisecond)
		}
		worst = max(worst, time.Since(begin))
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}
	return worst
}

// A TERM that arrives while the command starts must not leave the command running. The command
// writes its own pid and then becomes a sleep of 40 seconds or more, so a command that was missed
// outlives the wait below. Only that pid counts: a copy of the script that is only leaving, or a
// subshell of it that a loaded machine delays, also holds the marker in its command line.
func TestQueueScriptsTermDuringStart(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, name := range []string{"testslot.sh", "benchlock.sh"} {
		base := []string{scriptPath(t, name)}
		if name == "benchlock.sh" {
			base = append(base, "t")
		}
		latency := startLatency(t, name, base)
		for i := 0; i < 70; i++ {
			state := t.TempDir()
			env := scriptEnv(t, state)
			marker := fmt.Sprintf("%d.%03d", 40+len(name), i)
			pidFile := filepath.Join(state, "command.pid")
			args := append(append([]string{}, base...), "bash", "-c", `echo $$ > "$0"; exec sleep `+marker, pidFile)
			cmd := exec.Command("bash", args...)
			cmd.Env = env
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			// From a fifth of the start latency to one and a half times it.
			delay := latency * time.Duration(20+i*130/69) / 100
			time.Sleep(delay)
			_ = cmd.Process.Signal(syscall.SIGTERM)
			// The script stops its command and exits promptly; a script that cannot signal the
			// command's group keeps waiting for it for the 40 seconds that it sleeps.
			exited := make(chan struct{})
			go func() { _ = cmd.Wait(); close(exited) }()
			select {
			case <-exited:
			case <-time.After(10 * time.Second):
				_ = exec.Command("pkill", "-f", "sleep "+marker).Run()
				_ = cmd.Process.Kill()
				<-exited
				t.Fatalf("%s: still running 10 s after a TERM sent after %v", name, delay)
			}
			// Wait until the command that started is gone, or until nothing that could still start
			// it is left; give up after 5 s.
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if pid, alive := commandPid(pidFile, marker); pid != "" && !alive || pid == "" && lingering(marker) == "" {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if pid, alive := commandPid(pidFile, marker); alive {
				ps, _ := exec.Command("ps", "-o", "pid,ppid,pgid,stat,args", "-p", pid).Output()
				_ = exec.Command("pkill", "-f", "sleep "+marker).Run()
				t.Fatalf("%s: the command survived a TERM sent after %v and 5 s of waiting:\n%s", name, delay, ps)
			}
		}
	}
}

// The command's standard input is /dev/null: job control does not redirect it, and a queued
// command must not read the pipe or the terminal of its caller.
func TestQueueScriptsCommandStdinIsNull(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, name := range []string{"testslot.sh", "benchlock.sh"} {
		args := []string{scriptPath(t, name)}
		if name == "benchlock.sh" {
			args = append(args, "t")
		}
		cmd := exec.Command("bash", append(args, "cat")...)
		cmd.Env = scriptEnv(t, t.TempDir())
		cmd.Stdin = strings.NewReader("from the caller\n")
		out, err := cmd.Output()
		if err != nil || len(out) != 0 {
			t.Errorf("%s: cat printed %q (err %v), want nothing: its input must be /dev/null", name, out, err)
		}
	}
}

// Without perl, which the scripts need for their locks, they stop with a message instead of waiting
// for a lock that they can never take.
func TestQueueScriptsWithoutPerl(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	bin := t.TempDir()
	for _, tool := range []string{"bash", "dirname"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not installed", tool)
		}
		if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"testslot.sh", "benchlock.sh"} {
		args := []string{scriptPath(t, name)}
		if name == "benchlock.sh" {
			args = append(args, "t")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, filepath.Join(bin, "bash"), append(args, "true")...)
		cmd.Env = append(scriptEnv(t, t.TempDir()), "PATH="+bin)
		out, err := cmd.CombinedOutput()
		timedOut := ctx.Err() != nil
		cancel()
		if timedOut || err == nil || !strings.Contains(string(out), "perl is required") {
			t.Errorf("%s without perl: timed out %v, err %v, output %q, want a failure that says perl is required", name, timedOut, err, out)
		}
	}
}

// A claimer killed between making a slot and writing its pid leaves a slot without a pid. It is
// taken back once it is older than a minute, and not before: its claimer may still be writing.
func TestTestslotReclaimsEmptySlot(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, c := range []struct {
		name    string
		age     time.Duration
		blocked bool
	}{{"fresh", 0, true}, {"old", 3 * time.Minute, false}} {
		state := t.TempDir()
		env := append(scriptEnv(t, state), "TESTSLOT_N=1")
		slot := filepath.Join(state, "slots", "1")
		if err := os.MkdirAll(slot, 0o755); err != nil {
			t.Fatal(err)
		}
		past := time.Now().Add(-c.age)
		if err := os.Chtimes(slot, past, past); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), blockWait(c.blocked))
		cmd := exec.CommandContext(ctx, "bash", scriptPath(t, "testslot.sh"), "true")
		cmd.Env = env
		err := cmd.Run()
		timedOut := ctx.Err() != nil
		cancel()
		if c.blocked != timedOut {
			t.Errorf("%s empty slot: blocked=%v, want %v (err %v)", c.name, timedOut, c.blocked, err)
		}
	}
}

// mainScripts copies the queue scripts of the revision before the race fixes, kept in testdata,
// to a directory of their own, so that they run as an earlier checkout's copy runs beside these.
func mainScripts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"lib.sh", "testslot.sh", "benchlock.sh"} {
		b, err := os.ReadFile(filepath.Join("testdata", "queue-scripts-f454c9f3e", name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, name), string(b), 0o755)
	}
	return dir
}

// overlapWork is a command "work.sh RUN [NAME [SECONDS]]" that records how many copies of it run
// at once in RUN/counts, and its start and end under NAME in RUN/events. It runs 2 seconds.
func overlapWork(t *testing.T) (work, run string) {
	t.Helper()
	run = t.TempDir()
	work = filepath.Join(run, "work.sh")
	write(t, work, `#!/usr/bin/env bash
echo "start ${2:-}" >> "$1/events"
echo $$ > "$1/live.$$"
sleep "${3:-2}"
ls "$1" | grep -c '^live\.' >> "$1/counts"
rm -f "$1/live.$$"
echo "end ${2:-}" >> "$1/events"
`, 0o755)
	return work, run
}

// startQueued starts a queue script on the work command and returns once the command runs.
func startQueued(t *testing.T, env []string, run string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("bash", args...)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for begin := time.Now(); time.Since(begin) < 20*time.Second; time.Sleep(20 * time.Millisecond) {
		if m, _ := filepath.Glob(filepath.Join(run, "live.*")); len(m) > 0 {
			return cmd
		}
	}
	_ = cmd.Process.Kill()
	t.Fatalf("%v: the command did not start", args)
	return nil
}

// waitOverlap waits for the commands and fails when they did not all run or ran together.
func waitOverlap(t *testing.T, what, run string, want int, cmds ...*exec.Cmd) {
	t.Helper()
	waitConcurrency(t, what, run, want, 1, cmds...)
}

// waitConcurrency waits for the commands and fails when they did not all run, or when the most
// copies that ran at once is not most.
func waitConcurrency(t *testing.T, what, run string, want, most int, cmds ...*exec.Cmd) {
	t.Helper()
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	counts, _ := os.ReadFile(filepath.Join(run, "counts"))
	lines := strings.Fields(string(counts))
	if len(lines) != want {
		t.Fatalf("%s: %d of %d commands ran: %q", what, len(lines), want, lines)
	}
	seen := 0
	for _, l := range lines {
		c, _ := strconv.Atoi(l)
		seen = max(seen, c)
	}
	if seen != most {
		t.Errorf("%s: at most %d commands ran at once, want %d: %q", what, seen, most, lines)
	}
}

// A live holder started by an earlier checkout's script excludes a claimer of the current one, and
// the other way round, for the test slots and the heavy-run lock: lanes on older branches share the
// queue with the current scripts.
func TestQueueScriptsBesideEarlierVersion(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	old := mainScripts(t)
	for _, name := range []string{"testslot.sh", "benchlock.sh"} {
		for _, holderOld := range []bool{true, false} {
			state := t.TempDir()
			env := append(scriptEnv(t, state), "TESTSLOT_N=1")
			work, run := overlapWork(t)
			args := func(dir, tag string) []string {
				a := []string{filepath.Join(dir, name)}
				if name == "benchlock.sh" {
					a = append(a, tag)
				}
				return append(a, work, run)
			}
			cur := filepath.Dir(scriptPath(t, name))
			holder, claimer := args(cur, "cur"), args(old, "old")
			if holderOld {
				holder, claimer = args(old, "old"), args(cur, "cur")
			}
			h := startQueued(t, env, run, holder...)
			c := exec.Command("bash", claimer...)
			c.Env = env
			if err := c.Start(); err != nil {
				t.Fatal(err)
			}
			waitOverlap(t, fmt.Sprintf("%s, earlier holder %v", name, holderOld), run, 2, h, c)
		}
	}
}

// benchlockClass runs the current benchlock.sh in a class on the work command.
func benchlockClass(t *testing.T, env []string, class, tag string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("bash", append([]string{scriptPath(t, "benchlock.sh"), tag}, args...)...)
	cmd.Env = append(append([]string{}, env...), "BENCHLOCK_CLASS="+class)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

// waitQueued returns once a queued record of the tag exists.
func waitQueued(t *testing.T, state, tag string) {
	t.Helper()
	for begin := time.Now(); time.Since(begin) < 20*time.Second; time.Sleep(20 * time.Millisecond) {
		m, _ := filepath.Glob(filepath.Join(state, "runs", tag+".*"))
		for _, f := range m {
			if b, _ := os.ReadFile(f); strings.HasPrefix(string(b), "state=queued") {
				return
			}
		}
	}
	t.Fatalf("no queued record of %s", tag)
}

// events reads the start and end events of the work commands, in order.
func events(t *testing.T, run string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(run, "events"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// before reports whether event a comes before event b.
func before(ev []string, a, b string) bool {
	ia, ib := -1, -1
	for i, e := range ev {
		if e == a && ia < 0 {
			ia = i
		}
		if e == b && ib < 0 {
			ib = i
		}
	}
	return ia >= 0 && ib >= 0 && ia < ib
}

// Shared jobs run K at a time, and no more: four of them with K = 2 run two by two.
func TestBenchlockSharedSlots(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	state := t.TempDir()
	env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=2")
	work, run := overlapWork(t)
	var cmds []*exec.Cmd
	for i := 0; i < 4; i++ {
		cmds = append(cmds, benchlockClass(t, env, "shared", fmt.Sprintf("s%d", i), work, run))
	}
	waitConcurrency(t, "four shared jobs, K = 2", run, 4, 2, cmds...)
}

// An exclusive job waits for the shared holders, and a shared job waits for an exclusive holder.
func TestBenchlockExclusiveAndSharedExclude(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, first := range []string{"shared", "exclusive"} {
		state := t.TempDir()
		env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=2")
		work, run := overlapWork(t)
		second := map[string]string{"shared": "exclusive", "exclusive": "shared"}[first]
		h := benchlockClass(t, env, first, "h", work, run, "h")
		for begin := time.Now(); ; time.Sleep(20 * time.Millisecond) {
			if m, _ := filepath.Glob(filepath.Join(run, "live.*")); len(m) > 0 {
				break
			}
			if time.Since(begin) > 20*time.Second {
				t.Fatalf("%s holder did not start", first)
			}
		}
		c := benchlockClass(t, env, second, "c", work, run, "c")
		waitOverlap(t, first+" holder, then "+second, run, 2, h, c)
	}
}

// A queued exclusive job blocks the shared jobs queued after it, though slots are free (K = 3 leaves
// one even when the exclusive job is counted as taking one), so a stream
// of shared jobs cannot starve it. A record without class=, written by an earlier version, ranks
// as an exclusive one.
func TestBenchlockExclusiveIsNotStarved(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, waiter := range []string{"current", "earlier"} {
		state := t.TempDir()
		env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=3")
		work, run := overlapWork(t)
		h := benchlockClass(t, env, "shared", "s1", work, run, "s1", "3")
		for begin := time.Now(); ; time.Sleep(20 * time.Millisecond) {
			if m, _ := filepath.Glob(filepath.Join(run, "live.*")); len(m) > 0 {
				break
			}
			if time.Since(begin) > 20*time.Second {
				t.Fatal("shared holder did not start")
			}
		}
		var x *exec.Cmd
		if waiter == "current" {
			x = benchlockClass(t, env, "exclusive", "x", work, run, "x", "1")
		} else {
			x = exec.Command("bash", filepath.Join(mainScripts(t), "benchlock.sh"), "x", work, run, "x", "1")
			x.Env = env
			if err := x.Start(); err != nil {
				t.Fatal(err)
			}
		}
		waitQueued(t, state, "x")
		s2 := benchlockClass(t, env, "shared", "s2", work, run, "s2", "1")
		for _, cmd := range []*exec.Cmd{h, x, s2} {
			if err := cmd.Wait(); err != nil {
				t.Fatal(err)
			}
		}
		ev := events(t, run)
		if !before(ev, "end s1", "start x") || !before(ev, "end x", "start s2") {
			t.Errorf("%s exclusive waiter: events %q, want s1, then x, then s2", waiter, ev)
		}
	}
}

// Records and holders of the earlier version and of the current shared class share one queue: an
// earlier holder excludes a shared job, and an earlier waiter waits for the shared holders, which
// leave benchlock/ without an owner that it could take over.
func TestBenchlockSharedBesideEarlierVersion(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	old := filepath.Join(mainScripts(t), "benchlock.sh")
	for _, holderOld := range []bool{true, false} {
		state := t.TempDir()
		env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=2")
		work, run := overlapWork(t)
		var h, c *exec.Cmd
		if holderOld {
			h = startQueued(t, env, run, old, "old", work, run)
			c = benchlockClass(t, env, "shared", "cur", work, run)
		} else {
			h = benchlockClass(t, env, "shared", "cur", work, run)
			for begin := time.Now(); ; time.Sleep(20 * time.Millisecond) {
				if m, _ := filepath.Glob(filepath.Join(run, "live.*")); len(m) > 0 {
					break
				}
				if time.Since(begin) > 20*time.Second {
					t.Fatal("shared holder did not start")
				}
			}
			c = exec.Command("bash", old, "old", work, run)
			c.Env = env
			if err := c.Start(); err != nil {
				t.Fatal(err)
			}
		}
		waitOverlap(t, fmt.Sprintf("shared beside earlier version, earlier holder %v", holderOld), run, 2, h, c)
	}
}

// A shared holder killed without releasing its slot is taken back, and the next job runs.
func TestBenchlockReclaimsDeadSharedHolder(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	state := t.TempDir()
	env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=1")
	work, run := overlapWork(t)
	crashHolder(t, env, state, "shared", "dead")
	for _, class := range []string{"exclusive", "shared"} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		cmd := exec.CommandContext(ctx, "bash", scriptPath(t, "benchlock.sh"), "next", work, run, class, "0")
		cmd.Env = append(append([]string{}, env...), "BENCHLOCK_CLASS="+class)
		err := cmd.Run()
		cancel()
		if err != nil {
			t.Fatalf("%s job after a dead shared holder: %v", class, err)
		}
	}
}

// queue-status.sh names the lock's class and each record's.
func TestQueueStatusShowsClass(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	state := t.TempDir()
	env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=2")
	work, run := overlapWork(t)
	h := benchlockClass(t, env, "shared", "s", work, run, "s", "3")
	for begin := time.Now(); ; time.Sleep(20 * time.Millisecond) {
		if m, _ := filepath.Glob(filepath.Join(run, "live.*")); len(m) > 0 {
			break
		}
		if time.Since(begin) > 20*time.Second {
			t.Fatal("shared holder did not start")
		}
	}
	x := benchlockClass(t, env, "exclusive", "x", work, run, "x", "0")
	waitQueued(t, state, "x")
	out, _ := runIn(t, ".", env, "bash", scriptPath(t, "queue-status.sh"))
	for _, want := range []string{"lock: shared, 1 of 2 slots: s ", " shared\tstate=running", " exclusive\tstate=queued"} {
		if !strings.Contains(out, want) {
			t.Errorf("queue-status.sh output lacks %q:\n%s", want, out)
		}
	}
	_ = h.Wait()
	_ = x.Wait()
}

// A stray file in benchlock/ keeps the lock from being removed: the script keeps its command's
// exit status and reports the stray file in the stall log, which queue-status.sh shows.
func TestBenchlockReleaseKeepsStatusBesideStrayFile(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, class := range []string{"exclusive", "shared"} {
		state := t.TempDir()
		cmd := exec.Command("bash", scriptPath(t, "benchlock.sh"), "stray", "bash", "-c", `: > "$1/benchlock/stray"; exit 3`, "_", state)
		cmd.Env = append(scriptEnv(t, state), "BENCHLOCK_CLASS="+class)
		out, err := cmd.CombinedOutput()
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 3 {
			t.Errorf("%s: exit %v, want the command's 3; output %s", class, err, out)
		}
		log, _ := os.ReadFile(filepath.Join(state, "benchlock-stalls.log"))
		if !strings.Contains(string(log), "stray: stray.") || !strings.Contains(string(log), "holds: stray") {
			t.Errorf("%s: stall log %q does not report the stray file", class, log)
		}
	}
}

// crashHolder starts a holder of the class under tag and kills it as a crash would: the wrapper
// alone, with SIGKILL, so that it leaves its hold and its running record; then the group of its
// command, which this test started, as the running record names it.
func crashHolder(t *testing.T, env []string, state, class, tag string) {
	t.Helper()
	h := benchlockClass(t, env, class, tag, "sleep", "60")
	pgid := 0
	for begin := time.Now(); pgid == 0; time.Sleep(20 * time.Millisecond) {
		recs, _ := filepath.Glob(filepath.Join(state, "runs", tag+".*"))
		for _, r := range recs {
			b, _ := os.ReadFile(r)
			for _, f := range strings.Fields(string(b)) {
				if v, ok := strings.CutPrefix(f, "pgid="); ok {
					pgid, _ = strconv.Atoi(v)
				}
			}
		}
		if time.Since(begin) > 20*time.Second {
			_ = h.Process.Kill()
			t.Fatalf("%s holder did not start", class)
		}
	}
	if pgid <= 1 {
		t.Fatalf("bad group %d", pgid)
	}
	_ = h.Process.Kill()
	_ = h.Wait()
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

// A shared holder killed with SIGKILL while only waiters of the earlier version are queued: they
// cannot take its slot back, so queue-status.sh must report the dead holder and its record as
// stale and exit 1. The next current benchlock.sh takes the slot back and the queue moves.
func TestQueueStatusReportsDeadSharedHolder(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	state := t.TempDir()
	env := append(scriptEnv(t, state), "BENCHLOCK_SHARED=2")
	crashHolder(t, env, state, "shared", "dead")
	work, run := overlapWork(t)
	old := exec.Command("bash", filepath.Join(mainScripts(t), "benchlock.sh"), "old", work, run, "old", "0")
	old.Env = env
	if err := old.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Process.Kill() }()
	waitQueued(t, state, "old")
	time.Sleep(2 * time.Second) // several polls of the earlier waiter
	out, err := runIn(t, ".", env, "bash", scriptPath(t, "queue-status.sh"))
	if err == nil {
		t.Errorf("queue-status.sh exited 0 with a dead shared holder:\n%s", out)
	}
	for _, want := range []string{"STALE: shared holder dead ", "STALE: running record dead.", "STUCK", "queued: 1 running: 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("queue-status.sh output lacks %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(run, "events")); err == nil {
		t.Fatal("the earlier waiter ran while the dead holder's slot was held")
	}
	// A current job takes the slot back; then the earlier waiter runs.
	cur := exec.Command("bash", scriptPath(t, "benchlock.sh"), "cur", work, run, "cur", "0")
	cur.Env = env
	if out, err := cur.CombinedOutput(); err != nil {
		t.Fatalf("current job: %v\n%s", err, out)
	}
	if err := old.Wait(); err != nil {
		t.Fatalf("earlier waiter: %v", err)
	}
	out, err = runIn(t, ".", env, "bash", scriptPath(t, "queue-status.sh"))
	if err != nil || strings.Contains(out, "STALE") {
		t.Errorf("queue-status.sh after the queue moved: %v\n%s", err, out)
	}
}

// The backstop: a waiter of either class that sees the lock free for BENCHLOCK_STALL seconds while
// the ranking holds it back takes the lock and logs the stall. Here a live record that never takes
// the lock is ranked first: a process of a script named benchlock.sh that only sleeps.
func TestBenchlockStallBackstop(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, scriptsDeps...)
	needTools(t)
	for _, class := range []string{"shared", "exclusive"} {
		state := t.TempDir()
		env := append(scriptEnv(t, state), "BENCHLOCK_STALL=3")
		fake := filepath.Join(t.TempDir(), "benchlock.sh")
		write(t, fake, "sleep \"$2\"\n", 0o755)
		blocker := exec.Command("bash", fake, "blocker", "60")
		if err := blocker.Start(); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(state, "runs", fmt.Sprintf("blocker.%d", blocker.Process.Pid)),
			fmt.Sprintf("state=queued seq=0 tag=blocker wrapper=%d cwd=/ start=2000-01-01T00:00:00 cmd=sleep 60\n", blocker.Process.Pid), 0o644)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "bash", scriptPath(t, "benchlock.sh"), "late", "true")
		cmd.Env = append(append([]string{}, env...), "BENCHLOCK_CLASS="+class)
		out, err := cmd.CombinedOutput()
		cancel()
		_ = blocker.Process.Kill()
		_ = blocker.Wait()
		if err != nil {
			t.Fatalf("%s waiter behind a record that never runs: %v\n%s", class, err, out)
		}
		log, _ := os.ReadFile(filepath.Join(state, "benchlock-stalls.log"))
		if !strings.Contains(string(log), "stall: lock free") || !strings.Contains(string(log), class+") took it") {
			t.Errorf("%s: stall log %q does not record the stall", class, log)
		}
	}
}
