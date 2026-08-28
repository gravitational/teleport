#!/bin/bash
#
# This script generates a changelog for a release.
#
# It can optionally take two input variables: BASE_BRANCH: The base release
# branch to generate the changelog for. It will be of the form "branch/v*".
# BASE_TAG: The tag/version to generate the changelog from. It will be of the
# form "vXX.Y.Z", e.g. "v15.1.1"
#
# If neither are provided, the values will be automatically determined if
# possible:
# * The current branch will be used as the base branch if it matches the
#   pattern branch/v*
# * If the current branch is forked from a base branch, the base branch will be
#   used. e.g. if you create release/15.1.1 from branch/v15, the branch/v15
#   will be the base branch.
# * The base tag will be determined by running "make print-version" from the
#   root of the repo.
#
# Enterprise PR changelogs will be listed after the OSS changelogs. You need to
# determine if it is suitable to include them. If you do, remove the markdown
# link from each changelog when adding the changelog to CHANGELOG.md. These
# links wont work for the general public. Keep the links when adding the
# changelog to the release PR so that the enterprise PRs will link to the
# release PR.
#
# A changelog line may be marked with "NOCL:". This means there was no
# "Changelog:" line on the PR, and the PR title was used instead. You will
# likely need to reword these.
#
# If you reword changelogs, it is best to go to the source PR and change it
# there and then regenerate the changelog.
#
# Caveats:
# * If you update the "e" ref in your release PR, and you also run `make
#   changelog` from the release PR branch, if you have already updated the
#   version in the makefile, you will need to run `make changelog
#   BASE_TAG=X.Y.Z` as this script will determine the base tag to be the
#   current release version not the last released version.
#
# One preferred way of using this script is to run `make changelog` from the
# base branch and save it: `make changelog > /tmp/changelog`. If any PRs are
# merged to the base branch after you have created your release PR but before
# you have merged it, you can see any new entries with:
#
#     git checkout branch/vNN
#     diff -u /tmp/changelog $(make changelog)
#
# If there are changes, you can update your changelog and rebase your branch:
#
#     git pull # on branch/vNN
#     make changelog > /tmp/changelog
#     git checkout release/XX.Y.Z
#     git rebase branch/vNN
#     <include /tmp/changelog in CHANGELOG.md>
#     git add CHANGELOG.md && git commit --amend --no-edit && git push -f
#
# Ensure you update the PR body with the new changelog, doable on the command
# line with `gh`:
#
#     gh pr edit --body-file /tmp/changelog
#

# Set by check_prereq - either jq or gojq
JQ=

main() {
	set -eu

	check_prereq || exit 1

	local branch last_version
	branch=$(get_branch) || exit 1
	last_version=$(get_last_version) || exit 1

	since=$(git show -s --date=format:'%Y-%m-%dT%H:%M:%S%z' --format=%cd "${last_version}") || {
		echo 'Cant get timestamp of last release' >&2
		return 1
	}
	since_e=$(git -C e show -s --date=format:'%Y-%m-%dT%H:%M:%S%z' --format=%cd "${last_version}") || {
		echo 'Cant get enterprise timestamp of last release' >&2
		return 1
	}
	e_time=$(git log -n 1 --date=format:'%Y-%m-%dT%H:%M:%S%z' --format=%cd e) || {
		echo 'Cant get last modified time of "e" ref' >&2
		return 1
	}

	list_prs "${branch}" "${since}"
	printf '\nEnterprise:\n'
	(cd e && list_prs "${branch}" "${since_e}" "${e_time}")

	return 0
}

check_prereq() {
	if command -v jq >/dev/null 2>&1; then
		JQ=jq
		return 0
	fi
	if command -v gojq >/dev/null 2>&1; then
		JQ=gojq
		return 0
	fi

	echo 'jq or gojq not installed. Install gojq easily with:' >&2
	echo 'go install github.com/itchyny/gojq/cmd/gojq@latest' >&2
	return 1
}

get_branch() {
	# If BASE_BRANCH is set, just use that. Otherwise try to figure it out.
	if [[ -n "${BASE_BRANCH-}" ]]; then
		echo "${BASE_BRANCH}"
		return 0
	fi

	local ref branch
	ref=$(git symbolic-ref HEAD 2>/dev/null) || {
		echo 'Not on a branch' >&2
		return 1
	}
	branch="${ref#refs/heads/}"
	if [[ "${ref}" == "${branch}" ]]; then
		echo "Not on a branch: ${ref}" >&2
		return 1
	fi

	if [[ "${branch}" != branch/v* ]]; then
		# If we're already on the branch cut for the release, try to
		# determine the root branch name
		local fbranch
		fbranch=$(
			git branch \
				--list 'branch/v*' \
				--contains "$(git merge-base --fork-point HEAD)" \
				--format '%(refname:short)'
		)
		branch="${fbranch:-${branch}}" # Don't overwrite $branch with empty
	fi
	if ! [[ "${branch}" == branch/v* ]]; then
		echo "Not on a release branch: ${branch}" >&2
		return 1
	fi

	echo "${branch}"
	return 0
}

get_last_version() {
	# If BASE_TAG is set, just use that, otherwise figure out the last version
	if [[ -n "${BASE_TAG-}" ]]; then
		echo "${BASE_TAG}"
		return 0
	fi

	cd "$(git rev-parse --show-toplevel 2>/dev/null)" || {
		echo 'ugh. cant cd to repo root' >&2
		return 1
	}
	local last_version
	last_version=$(make -s print-version) || {
		echo 'Cant get last released version' >&2
		return 1
	}

	echo "v${last_version}"
	return 0
}

list_prs() {
	local branch="$1" from="$2" to="${3-}"
	local merged_query
	if [[ -z "${to}" ]]; then
		merged_query="merged:>${from}"
	else
		merged_query="merged:${from}..${to}"
	fi

	local gh_query="base:${branch} ${merged_query} -label:no-changelog"

	# shellcheck disable=SC2016 # We're not trying to expand in single quotes
	jq_expr='
	  def extract_cl: gsub("\r"; "") | split("\n") | [.[] | scan("^[Cc]hangelog: +(.*)$") | .[]];
	  def promote_title: {number, url, changelog: (if .changelog == [] then ["NOCL: " + .title] else .changelog end)};
	  def flatten_changes: . as $pr | .changelog[] | $pr + {changelog: .};
	  def clean: sub("\\s*$"; "") | if . | endswith(".") then . else . + "." end;
	  def as_entry: "* \(.changelog | clean) [#\(.number)](\(.url))";
	  map({number, url, title, changelog: .body | extract_cl}) |
	  map(promote_title) |
	  map(flatten_changes) |
	  map(as_entry) |
	  join("\n")
	'

	gh pr list \
		--search "${gh_query}" \
		--limit 200 \
		--json number,url,title,body |
		"${JQ}" -r "${jq_expr}"
}

# Only run main if executed as a script and not sourced.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
