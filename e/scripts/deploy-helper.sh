#!/bin/bash

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

    # Nightly base-image tags are already long enough to exceed tenant version
    # naming limits, so collapse both release-branch nightly tags
    # (e.g. 18.7.2-nightly...) and master nightly tags
    # (e.g. 19.0.0-dev-nightly...) into a shorter dotted-tri form.
    if [[ "${base_image_tag}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-dev)?-nightly([0-9]{8})-[0-9]+-[0-9a-f]+$ ]]; then
        local major="${BASH_REMATCH[1]}"
        local minor="${BASH_REMATCH[2]}"
        local patch="${BASH_REMATCH[3]}"
        local nightly_date="${BASH_REMATCH[5]}"
        local tenant_short="${tenant:0:16}"
        echo "${major}.${minor}.${patch}-nightly${nightly_date}-${tenant_short}-${timestamp}"
        return
    fi

    echo "${base_image_tag}-${tenant}-${commit_short}-${timestamp}"
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
