#!/bin/zsh -f
# Mutation control of hack/affected-tests.sh: for each commit C, undo C's change to non-test source and docs
# (keeping its tests and goldens), run the tests of the heavy packages and of the selector's output, and report
# the failing packages that the selector did not name. Usage: affected-tests-control.sh <worktree-dir> <commit>...
# The tests run in the default tier. A failing package outside the selection is rerun at C unbroken to tell a
# missed dependency from a test that fails there too.
export PATH=/opt/homebrew/bin:$HOME/.nodenv/shims:$HOME/go/bin:/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH
sel=$(cd $(dirname $0) && pwd)/affected-tests.sh
W=$1; shift
M=zeroparse.dev/zp
heavy=(internal/gen internal/gen/gogen internal/gen/tsgen internal/engine internal/runtimetest internal/complexitytest spec/language internal/compile)
for c in "$@"; do
  d=$W/ctl-$c; rm -rf $d; git worktree add -q --detach $d $c || continue
  ( cd $d
    files=(${(f)"$(git diff --name-only $c~1 $c | grep -v '_test\.go$' | grep -v '/testdata/')"})
    for f in $files; do
      if git cat-file -e $c~1:$f 2>/dev/null; then git checkout -q $c~1 -- $f; else git rm -q -f $f; fi
    done
    git -c user.email=x@x -c user.name=x -c commit.gpgsign=false commit -q -a -m broken --allow-empty
    out=$($sel $c HEAD)
    pk=(${(f)"$(echo $out | grep -v '^#')"})
    flags=$(echo $out | grep '^#' | tr '\n' ' ')
    # Run every package except the heavy ones that the selector names: a failure in a package the selector did not
    # name is a miss, and the selected cheap packages show whether the break is caught at all.
    allp=(${(f)"$(go list ./... ./runtime/... | sort -u)"})
    run=()
    for p in $allp; do
      r=${p#$M/}; [ $p = $M ] && r=.
      if (( ${heavy[(Ie)$r]} )) && print -rl -- $pk | grep -qx $p; then continue; fi
      run+=$p
    done
    echo "== $c $(git log -1 --format=%s $c | cut -c1-50) | reverted: $files | selected ${#pk} pkgs $flags | ran ${#run}"
    typeset -a fail; fail=()
    for p in ${(f)"$(go test -count=1 $run 2>&1 | grep -E '^FAIL\s+\S+' | awk '{print $2}' | sort -u)"}; do fail+=$p; done
    if echo $flags | grep -q site; then (cd site && go test -count=1 ./... >/dev/null 2>&1) || fail+=SITE; fi
    echo "   failing: ${fail:-none}"
    for p in $fail; do
      [ $p = SITE ] && { echo $flags | grep -q site || echo "   MISSED: site"; continue; }
      print -rl -- $pk | grep -qx $p || echo "   NOT SELECTED: $p (a miss unless it fails at C unbroken too)"
    done
  )
  git worktree remove --force $d
done
