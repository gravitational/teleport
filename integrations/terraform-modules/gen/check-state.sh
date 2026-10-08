#!/bin/bash

set -euo pipefail

state_root="$(pwd -P)"
state_check_dir="$(mktemp -d)"
trap 'rm -rf "${state_check_dir}"' EXIT

# Inspect state before cleanup removes any files. Include backups and workspace
# state because they may be the only recovery copy.
find "${state_root}" -type f -name 'terraform.tfstate*' -print0 > "${state_check_dir}/state-files"

while IFS= read -r -d '' state_file; do
    # A backup can still contain resources after a successful destroy. Prefer
    # the current state when present, and inspect it below before allowing
    # cleanup. Do not trust a missing or zero-byte current state over a backup.
    if [[ "${state_file}" == *.backup ]]; then
        current_state_file="${state_file%.backup}"
        if [[ -f "${current_state_file}" && -s "${current_state_file}" ]]; then
            state_file="${current_state_file}"
        fi
    fi

    [[ ! -s "${state_file}" ]] && continue

    # Use an empty configuration and the default workspace so an existing
    # backend or workspace cannot redirect the lookup away from this file.
    if ! resources="$(TF_DATA_DIR="${state_check_dir}/.terraform" TF_WORKSPACE=default \
        terraform -chdir="${state_check_dir}" state list -state="${state_file}")"; then
        printf 'Cannot clean: unable to inspect Terraform state "%s". No cleanup files were deleted.\n' "${state_file}" >&2
        printf 'Restore or repair this state file, or move it to a safe location outside this directory, then retry make clean.\n' >&2
        exit 1
    fi

    if [[ -n "${resources}" ]]; then
        printf 'Cannot clean: Terraform state "%s" still records deployed resources. No cleanup files were deleted.\n' "${state_file}" >&2
        printf 'Run terraform destroy in the owning configuration and workspace before retrying make clean.\n' >&2
        printf 'If retaining the resources, migrate the state to a safe location outside this directory first.\n' >&2
        printf 'For a backup, verify that the resources were destroyed or are managed by another state, then archive the backup outside this directory.\n' >&2
        exit 1
    fi
done < "${state_check_dir}/state-files"
