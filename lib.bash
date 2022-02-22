#!/bin/false

# returns 0 if the given string is a 40 character hexadecimal string
looks_like_git_commit() {
  if [[ "$1" =~ ^[0-9a-f]{40}$ ]]; then
    return 0
  fi
  return 1
}

# compares head and base commits and emits a status message
# status values: 'ahead', 'behind', 'identical', 'diverged', "not_found"
# does not error on not_found
compare_github_commits() {
  local base_commit="$1"
  local head_commit="$2"
  local request_path="repos/{owner}/{repo}/compare/$base_commit...$head_commit"
  local result
  result="$(gh api "$request_path" --jq '.status' 2>/dev/null)" || true
  [ -n "$result" ] && echo "$result" || echo "not_found"
}

# emits the tags that look like git commits that point to the head of the given bsr track
bsr_track_head_git_commit_hashes() {
  local bsr_repository="$1"
  local track_name="$2"
  local track_reference="${bsr_repository}:${track_name}"
  local output_json
  output_json="$(buf beta registry commit get "${track_reference}" --format json)"
  for tag in $(echo "${output_json}" | jq -r '.tags[].name'); do
    if looks_like_git_commit "${tag}"; then
      echo "${tag}"
    fi
  done
}
