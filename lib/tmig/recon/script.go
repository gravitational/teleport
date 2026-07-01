// Package recon implements the SSH probe that runs on target hosts to gather
// migration-relevant facts. The probe is read-only, does not access secrets,
// and produces shellcheck-clean bash.
package recon

// RenderScript produces a shellcheck-clean, read-only probe script.
// All interpolations are quoted. No writes. No secrets.
func RenderScript() string {
	return `#!/bin/bash
set -euo pipefail

# tmig recon probe — read-only, no secrets, shellcheck-clean
# Output: key=value pairs on stdout, one per line, prefix TMIG_

printf 'TMIG_OS=%s\n' "$(uname -s)"

# systemd check
if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files teleport.service >/dev/null 2>&1; then
    printf 'TMIG_SYSTEMD=true\n'
else
    printf 'TMIG_SYSTEMD=false\n'
fi

# teleport-update presence
if command -v teleport-update >/dev/null 2>&1; then
    printf 'TMIG_TELEPORT_UPDATE=true\n'
else
    printf 'TMIG_TELEPORT_UPDATE=false\n'
fi

# Config path detection
CONFIG_PATH="/etc/teleport.yaml"
if [ -f "${CONFIG_PATH}" ]; then
    printf 'TMIG_CONFIG_PATH=%s\n' "${CONFIG_PATH}"
    printf 'TMIG_CONFIG_READABLE=true\n'
else
    # Check common alternatives
    for candidate in /etc/teleport/teleport.yaml /opt/teleport/teleport.yaml; do
        if [ -f "${candidate}" ]; then
            CONFIG_PATH="${candidate}"
            break
        fi
    done
    printf 'TMIG_CONFIG_PATH=%s\n' "${CONFIG_PATH}"
    if [ -r "${CONFIG_PATH}" ]; then
        printf 'TMIG_CONFIG_READABLE=true\n'
    else
        printf 'TMIG_CONFIG_READABLE=false\n'
    fi
fi

# Root/sudo check
if [ "$(id -u)" = "0" ]; then
    printf 'TMIG_ROOT=true\n'
elif sudo -n true 2>/dev/null; then
    printf 'TMIG_ROOT=true\n'
else
    printf 'TMIG_ROOT=false\n'
fi

# Join method from config
JOIN_METHOD=""
if [ -r "${CONFIG_PATH}" ]; then
    JOIN_METHOD="$(grep -m1 'method:' "${CONFIG_PATH}" 2>/dev/null | sed 's/.*method:[[:space:]]*//' | tr -d '"' || true)"
    if [ -z "${JOIN_METHOD}" ]; then
        if grep -q 'auth_token:' "${CONFIG_PATH}" 2>/dev/null; then
            JOIN_METHOD="token"
        fi
    fi
fi
printf 'TMIG_JOIN_METHOD=%s\n' "${JOIN_METHOD}"

# Services
SERVICES=""
if [ -r "${CONFIG_PATH}" ]; then
    for svc in ssh_service kubernetes_service app_service db_service windows_desktop_service discovery_service; do
        if grep -q "${svc}:" "${CONFIG_PATH}" 2>/dev/null; then
            enabled="$(sed -n "/${svc}:/,/^[a-z]/{ /enabled:/p; }" "${CONFIG_PATH}" | head -1 | grep -o 'true\|yes' || true)"
            if [ -n "${enabled}" ]; then
                if [ -z "${SERVICES}" ]; then
                    SERVICES="${svc}"
                else
                    SERVICES="${SERVICES},${svc}"
                fi
            fi
        fi
    done
fi
printf 'TMIG_SERVICES=%s\n' "${SERVICES}"

# Listen addresses
LISTEN_ADDRS=""
if [ -r "${CONFIG_PATH}" ]; then
    LISTEN_ADDRS="$(grep -E 'listen_addr:|tunnel_listen_addr:' "${CONFIG_PATH}" 2>/dev/null | sed 's/.*:[[:space:]]*//' | tr -d '"' | paste -sd ',' - || true)"
fi
printf 'TMIG_LISTEN_ADDRS=%s\n' "${LISTEN_ADDRS}"

# Binary version
BINARY_VERSION=""
if command -v teleport >/dev/null 2>&1; then
    BINARY_VERSION="$(teleport version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)"
fi
printf 'TMIG_BINARY_VERSION=%s\n' "${BINARY_VERSION}"

# Install kind
INSTALL_KIND="other"
if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files teleport.service >/dev/null 2>&1; then
    INSTALL_KIND="systemd"
elif [ -f "/.dockerenv" ] || grep -q 'docker\|containerd' /proc/1/cgroup 2>/dev/null; then
    INSTALL_KIND="container"
elif command -v supervisorctl >/dev/null 2>&1 && supervisorctl status teleport >/dev/null 2>&1; then
    INSTALL_KIND="supervisor"
fi
printf 'TMIG_INSTALL_KIND=%s\n' "${INSTALL_KIND}"
`
}
