#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! $1 =~ ^[1-9][0-9]*$ ]]; then
  echo "usage: scripts/promote-main.sh <main-pull-request-number>" >&2
  exit 2
fi

repository=puffball1567/kinmokusei
pull_request=$1

for url in "$(git remote get-url origin)" "$(git remote get-url --push origin)"; do
  case "$url" in
    "https://github.com/$repository"|"https://github.com/$repository.git"|\
    "git@github.com:$repository.git"|"git@github.com:$repository"|\
    "ssh://git@github.com/$repository.git"|"ssh://git@github.com/$repository")
      ;;
    *)
      echo "origin must point to $repository." >&2
      exit 1
      ;;
  esac
done

read_promotion() {
  gh pr view "$pull_request" --repo "$repository" \
    --json state,baseRefName,headRefName,headRefOid,isCrossRepository,mergeCommit,statusCheckRollup \
    --jq '
      select(.baseRefName == "main" and .headRefName == "devel" and (.isCrossRepository | not))
      | select(.state == "OPEN" or .state == "MERGED")
      | select(any(.statusCheckRollup[]; .name == "Require approved main source" and .conclusion == "SUCCESS"))
      | select(any(.statusCheckRollup[]; .name == "Verify successful devel CI" and .conclusion == "SUCCESS"))
      | select(any(.statusCheckRollup[]; .name == "Required compatibility" and .conclusion == "SUCCESS"))
      | [.state, .headRefOid, (.mergeCommit.oid // "-")] | @tsv
    '
}

read_heads() {
  local heads
  heads=$(git ls-remote --refs origin refs/heads/main refs/heads/devel)
  current_main=$(awk '$2 == "refs/heads/main" { print $1 }' <<< "$heads")
  current_devel=$(awk '$2 == "refs/heads/devel" { print $1 }' <<< "$heads")
}

promotion=$(read_promotion)
if [[ -z "$promotion" ]]; then
  echo "PR #$pull_request must promote this repository's devel to main and pass its checks." >&2
  exit 1
fi
read -r state verified_devel merge_commit <<< "$promotion"

if [[ "$state" == OPEN ]]; then
  read_heads
  if [[ "$current_devel" != "$verified_devel" ]]; then
    echo "devel advanced after the PR was verified. Verify the current PR head before merging." >&2
    exit 1
  fi
  gh pr merge "$pull_request" --repo "$repository" --merge --match-head-commit "$verified_devel"
  promotion=$(read_promotion)
  if [[ -z "$promotion" ]]; then
    echo "Could not confirm the merge. Run this command again after it completes." >&2
    exit 1
  fi
  read -r state verified_devel merge_commit <<< "$promotion"
fi

read_heads
if [[ "$state" != MERGED || "$current_main" != "$merge_commit" ]]; then
  echo "PR #$pull_request must be merged at the current main head." >&2
  exit 1
fi
if [[ "$current_devel" == "$merge_commit" ]]; then
  echo "main and devel already point to $merge_commit."
  exit 0
fi
if [[ "$current_devel" != "$verified_devel" ]]; then
  echo "devel advanced after the merge. Synchronize the histories through a PR before the next promotion." >&2
  exit 1
fi

git fetch --no-tags origin refs/heads/main refs/heads/devel
if ! git merge-base --is-ancestor "$verified_devel" "$merge_commit" ||
   ! git diff --quiet "$verified_devel" "$merge_commit"; then
  echo "main is not a content-preserving fast-forward of the verified devel head." >&2
  exit 1
fi

git push --no-force origin "$merge_commit:refs/heads/devel"
read_heads
if [[ "$current_main" != "$merge_commit" || "$current_devel" != "$merge_commit" ]]; then
  echo "Branch heads changed during synchronization. Check both branches before the next promotion." >&2
  exit 1
fi
echo "main and devel now point to $merge_commit."
