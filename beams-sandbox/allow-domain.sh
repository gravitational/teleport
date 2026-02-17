#!/bin/bash
# Add a domain to the network allowlist

set -e

DOMAIN="$1"
ALLOWED_FILE="/workspace/.beams/allowed-domains"

if [ -z "$DOMAIN" ]; then
    echo "Usage: $0 <domain>"
    exit 1
fi

# Strip protocol if present
DOMAIN=$(echo "$DOMAIN" | sed 's|https\?://||' | sed 's|/.*||')

# Resolve domain to IPs
IPS=$(dig +short "$DOMAIN" | grep -E '^[0-9.]+$' || true)

if [ -z "$IPS" ]; then
    echo "Warning: Could not resolve $DOMAIN"
    exit 1
fi

# Add iptables rules for each IP
for ip in $IPS; do
    iptables -I OUTPUT 1 -d "$ip" -j ACCEPT
done

# Track the allowed domain
mkdir -p /workspace/.beams
if ! grep -q "^${DOMAIN}$" "$ALLOWED_FILE" 2>/dev/null; then
    echo "$DOMAIN" >> "$ALLOWED_FILE"
fi

