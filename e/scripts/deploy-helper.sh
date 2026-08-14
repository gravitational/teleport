#!/bin/bash

MAX_TARGET_IMAGE_TAG_LENGTH=60

# Truncation can leave a dangling separator, which in turn creates awkward
# doubled-hyphen tags when segments are rejoined.
# Example: "test1-" becomes "test1" so the final tag is
# "19.0.0-prealpha.2-test1-<sha>-<timestamp>", not "...-test1--<sha>-...".
function trim_trailing_hyphens() {
    local value="$1"
    while [[ "${value}" == *- ]]; do
        value="${value%-}"
    done
    echo "${value}"
}

# Build the final tag while omitting the tenant separator when the tenant
# segment has been truncated away entirely.
# Example: prefix "19.0.0-prealpha.2", empty tenant, suffix
# "-4424d04dc07-20260323-1332" becomes
# "19.0.0-prealpha.2-4424d04dc07-20260323-1332".
function join_image_tag_parts() {
    local prefix_base="$1"
    local tenant_segment="$2"
    local suffix="$3"

    if [[ -n "${tenant_segment}" ]]; then
        echo "${prefix_base}-${tenant_segment}${suffix}"
        return
    fi

    echo "${prefix_base}${suffix}"
}

function ensure_teleport_binary() {
    local binary_path="${BUILDDIR}/teleport"

    if [[ ! -f "$binary_path" ]]; then
        fail_on_exit_code "Expected teleport binary at \"$binary_path\"." 1
    fi

    local file_output
    file_output=$(file -b "$binary_path")
    fail_on_exit_code "Unable to inspect \"$binary_path\" with file."
    if ! (echo "$file_output" | grep -qi 'ELF' && echo "$file_output" | grep -qi 'x86-64'); then
        fail_on_exit_code "Invalid binary architecture for \"$binary_path\". Expected linux/amd64 (ELF x86-64), got: $file_output" 1
    fi

    local go_build_info
    go_build_info=$(go version -m "$binary_path" 2>/dev/null)
    fail_on_exit_code "Unable to read Go build info from \"$binary_path\"."
    if ! echo "$go_build_info" | grep -Eq '^[[:space:]]*path[[:space:]]+github.com/gravitational/teleport(/|$).*/tool/teleport$'; then
        fail_on_exit_code "\"$binary_path\" is not a teleport binary (expected main package path ending in /tool/teleport)." 1
    fi
}

function build_target_image_tag() {
    local base_image_tag="$1"
    local tenant="$2"
    local commit_short="$3"
    local timestamp="$4"
    local suffix="-${commit_short}-${timestamp}"
    local prefix_base
    local tenant_segment

    # Nightly base-image tags are already long enough to exceed tenant version
    # naming limits, so collapse both release-branch nightly tags
    # (e.g. 18.7.2-nightly...) and master nightly tags
    # (e.g. 19.0.0-dev-nightly...) into a shorter dotted-tri form.
    if [[ "${base_image_tag}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-dev)?-nightly([0-9]{8})-[0-9]+-[0-9a-f]+$ ]]; then
        local major="${BASH_REMATCH[1]}"
        local minor="${BASH_REMATCH[2]}"
        local patch="${BASH_REMATCH[3]}"
        # Capture 5 is the nightly date in YYYYMMDD form.
        local nightly_date="${BASH_REMATCH[5]}"
        prefix_base="${major}.${minor}.${patch}-nightly${nightly_date}"
        # Apply a small early cap so nightly tags remain stable and readable
        # before the general length-budget pass below.
        tenant_segment="${tenant:0:16}"
    else
        prefix_base="${base_image_tag}"
        tenant_segment="${tenant}"
    fi

    # Keep the common path fast when the fully rendered tag already fits.
    # Example: "18.7.3-short-1234567-20260323-1332" is already under the cap, so
    # it is returned unchanged.
    tenant_segment=$(trim_trailing_hyphens "${tenant_segment}")
    local candidate
    candidate=$(join_image_tag_parts "${prefix_base}" "${tenant_segment}" "${suffix}")
    if (( ${#candidate} <= MAX_TARGET_IMAGE_TAG_LENGTH )); then
        echo "${candidate}"
        return
    fi

    # Preserve the version and suffix first, then spend the remaining budget on
    # the tenant segment.
    # Example: "19.0.0-prealpha.2-test1-19-pretestplan-4424d04dc07-20260323-1332"
    # first becomes something like
    # "19.0.0-prealpha.2-test1-19-pretest-4424d04dc07-20260323-1332".
    local tenant_max_length=$(( MAX_TARGET_IMAGE_TAG_LENGTH - ${#prefix_base} - ${#suffix} - 1 ))
    if (( tenant_max_length > 0 )); then
        tenant_segment="${tenant_segment:0:tenant_max_length}"
        tenant_segment=$(trim_trailing_hyphens "${tenant_segment}")
        candidate=$(join_image_tag_parts "${prefix_base}" "${tenant_segment}" "${suffix}")
        if (( ${#candidate} <= MAX_TARGET_IMAGE_TAG_LENGTH )); then
            echo "${candidate}"
            return
        fi
    fi

    # If the tag is still too long, preserve the commit/timestamp suffix and
    # trim the variable-width prefix. For semver prereleases, keep the dotted
    # tri intact and only shorten the prerelease payload.
    # Example: "19.0.0-superlongprereleaseidentifier-tenant-abcdef12345-20260323-1332"
    # becomes
    # "19.0.0-superlongprereleaseidentifi-abcdef12345-20260323-1332".
    local prefix_base_max_length=$(( MAX_TARGET_IMAGE_TAG_LENGTH - ${#suffix} ))
    if (( prefix_base_max_length <= 0 )); then
        # Degenerate guard: in normal use the suffix should fit comfortably, but
        # still keep the output bounded if the suffix ever exceeds the budget.
        echo "${suffix:1:MAX_TARGET_IMAGE_TAG_LENGTH}"
        return
    fi

    if [[ "${prefix_base}" =~ ^([0-9]+\.[0-9]+\.[0-9]+)-(.*)$ ]]; then
        local version_core="${BASH_REMATCH[1]}"
        local prerelease="${BASH_REMATCH[2]}"
        local prerelease_max_length=$(( prefix_base_max_length - ${#version_core} - 1 ))
        if (( prerelease_max_length > 0 )); then
            prefix_base="${version_core}-${prerelease:0:prerelease_max_length}"
            prefix_base="${prefix_base%-}"
        else
            prefix_base="${version_core}"
        fi
    else
        prefix_base="${prefix_base:0:prefix_base_max_length}"
        prefix_base="${prefix_base%-}"
    fi

    # After shortening the prefix, re-spend any newly available space on the
    # tenant segment before falling back to prefix+suffix only. At this point
    # tenant_segment still contains the original tenant candidate from the
    # earlier pass, so it can be retried against the new prefix budget.
    # Example: a nightly tag such as
    # "19.0.0-nightly20260312-test1-master-1-4424d04dc07-20260323-1332"
    # is shortened to
    # "19.0.0-nightly20260312-test1-maste-4424d04dc07-20260323-1332".
    tenant_max_length=$(( MAX_TARGET_IMAGE_TAG_LENGTH - ${#prefix_base} - ${#suffix} - 1 ))
    if (( tenant_max_length > 0 )); then
        tenant_segment="${tenant_segment:0:tenant_max_length}"
        tenant_segment=$(trim_trailing_hyphens "${tenant_segment}")
        join_image_tag_parts "${prefix_base}" "${tenant_segment}" "${suffix}"
        return
    fi

    echo "${prefix_base}${suffix}"
}

function resolve_dev_base_image_for_master() {
    local current_version="$1"
    local base_image_tag_was_set="$2"
    local base_image_repo_was_set="$3"
    local base_image_repo="$4"

    if [[ "${current_version}" != *-dev* || -n "${base_image_tag_was_set}" ]]; then
        return
    fi

    local branch
    branch=$(git -C .. rev-parse --abbrev-ref HEAD 2>/dev/null)
    if [[ "${branch}" != "master" ]]; then
        return
    fi

    local resolved_base_repo="$base_image_repo"
    if [[ -z "${base_image_repo_was_set}" ]]; then
        resolved_base_repo="public.ecr.aws/gravitational-staging/teleport-ent-distroless"
    fi

    local repo_path="${resolved_base_repo#public.ecr.aws/}"
    local registry_alias="${repo_path%%/*}"
    local repository_name="${repo_path#*/}"
    local nightly_prefix="${current_version}-nightly"

    local token
    token=$(curl -fsS 'https://public.ecr.aws/token/' | jq -r '.token // empty')
    if [[ -z "${token}" ]]; then
        error "Could not retrieve a token from public.ecr.aws."
        echo "Set BASE_IMAGE_REPO and BASE_IMAGE_TAG manually."
        exit 1
    fi

    local latest_tag
    latest_tag=$(curl -fsS -H "Authorization: Bearer ${token}" \
        "https://public.ecr.aws/v2/${registry_alias}/${repository_name}/tags/list" \
        | jq -r --arg prefix "${nightly_prefix}" '
            .tags[]?
            | select(startswith($prefix) and test("-nightly[0-9]{8}-[0-9]+-[0-9a-f]+$"))
        ' \
        | sort -rV \
        | head -n 1)

    if [[ -z "${latest_tag}" ]]; then
        error "Could not resolve latest nightly for version prefix \"${current_version}-nightly\" from \"${resolved_base_repo}\"."
        echo "Set BASE_IMAGE_REPO and BASE_IMAGE_TAG manually, e.g.:"
        echo "  BASE_IMAGE_REPO=public.ecr.aws/gravitational-staging/teleport-ent-distroless \\"
        echo "  BASE_IMAGE_TAG=19.0.0-dev-nightlyYYYYMMDD-1-<sha> \\"
        echo "  make TENANT=yourtenant create-cloud"
        exit 1
    fi

    BASE_IMAGE_REPO="${resolved_base_repo}"
    BASE_IMAGE_TAG="${latest_tag}"
    echo "-> Auto-resolved master/dev base image: \"${BASE_IMAGE_REPO}:${BASE_IMAGE_TAG}\""
}
