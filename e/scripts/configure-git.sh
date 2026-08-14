#!/usr/bin/env bash

# Enable SOPS diffing
# If SOPS isn't installed, this will do nothing
git config diff.sopsdiffer.textconv "sops decrypt"

# Add shared pre-commit hooks, if they don't already exist
HOOK_PATH=".git/hooks/pre-commit"

if [[ ! -f "${HOOK_PATH}" ]]; then
    echo "#!/usr/bin/env bash" > "${HOOK_PATH}"
    chmod +x "${HOOK_PATH}"
fi

HOOK_CALL='./scripts/shared-pre-commit.sh'
if ! grep -q "${HOOK_CALL}" "${HOOK_PATH}"; then
    echo "${HOOK_CALL}" >> "${HOOK_PATH}"
fi

# Warn if SOPS isn't installed

if ! command -v sops > /dev/null; then
    cat <<- EOF >&2
		SOPS not installed or available under \$PATH.
		Install with \`brew install sops\` or platform equivalent.
		EOF
fi
