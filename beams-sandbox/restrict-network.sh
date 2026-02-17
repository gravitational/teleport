#!/bin/bash
# Simulate Teleport Beams cloud network restrictions
# - DNS queries work
# - Only LLM APIs (Anthropic, OpenAI) are accessible
# - Everything else gets connection refused

set -e

echo "Creating sandbox..."

# Flush existing rules
iptables -F OUTPUT 2>/dev/null || true

# Allow loopback
iptables -A OUTPUT -o lo -j ACCEPT

# Allow DNS queries (UDP port 53)
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Resolve and allow Anthropic API
ANTHROPIC_IPS=$(dig +short api.anthropic.com | grep -E '^[0-9.]+$' || echo "")
for ip in $ANTHROPIC_IPS; do
    iptables -A OUTPUT -d "$ip" -j ACCEPT
done

# Allow claude.ai for installer/updates
CLAUDE_IPS=$(dig +short claude.ai | grep -E '^[0-9.]+$' || echo "")
for ip in $CLAUDE_IPS; do
    iptables -A OUTPUT -d "$ip" -j ACCEPT
done

# Resolve and allow OpenAI API
OPENAI_IPS=$(dig +short api.openai.com | grep -E '^[0-9.]+$' || echo "")
for ip in $OPENAI_IPS; do
    iptables -A OUTPUT -d "$ip" -j ACCEPT
done

# Allow established connections (for DNS responses)
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Reject TCP with reset (immediate connection refused) and UDP with icmp
iptables -A OUTPUT -p tcp -j REJECT --reject-with tcp-reset
iptables -A OUTPUT -p udp -j REJECT --reject-with icmp-port-unreachable

exec runuser -u beams -- "${@:-/bin/bash}"
