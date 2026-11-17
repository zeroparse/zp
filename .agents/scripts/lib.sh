#!/usr/bin/env bash
# Shared by the scripts in this directory; source it, do not run it.
#
# ZP_AGENT_STATE is the per-machine coordination directory. Default:
# ${XDG_STATE_HOME:-$HOME/.local/state}/zp-team. Layout:
#   landing-train.md   the live queue of candidates
#   wt/<name>/         worktrees
#   runs/ slots/ benchlock/ benchlock.seq benchlock-stalls.log   run queue (written by the scripts only)
#   gates/<tag>.log    gate logs
# BENCHLOCK_DIR and TESTSLOT_DIR still override the queue paths, for a state directory laid out differently.

zp_state=${ZP_AGENT_STATE:-${XDG_STATE_HOME:-$HOME/.local/state}/zp-team}
export ZP_AGENT_STATE=$zp_state

die() { echo "${0##*/}: $*" >&2; exit 1; }

# zp_repo_root prints the top level of the checkout that holds this script's directory.
zp_repo_root() { (cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P); }

# zp_branch_ok BRANCH: the branch prefixes of AGENTS.md.
zp_branch_ok() { case $1 in feat/* | fix/* | perf/* | docs/*) return 0 ;; *) return 1 ;; esac; }
