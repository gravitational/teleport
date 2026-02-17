#!/bin/bash
# Flexible domain allowlist management for beams-sandbox
# Usage: ./allow.sh --domain abc.com --domain def.com --python-dev

set -e

DOMAINS=()
PYTHON_DEV=false
GITHUB=false
NPM_DEV=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --domain)
            DOMAINS+=("$2")
            shift 2
            ;;
        --python-dev)
            PYTHON_DEV=true
            shift
            ;;
        --github)
            GITHUB=true
            shift
            ;;
        --npm-dev)
            NPM_DEV=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--domain <domain>]... [--python-dev] [--github]"
            exit 1
            ;;
    esac
done

# Check if the beams-sandbox container is running
CONTAINER_NAME="beams-sandbox-instance"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "❌ Error: Sandbox is not running"
    echo "Start it with: make run (in another terminal)"
    exit 1
fi

echo "📦 Using container: $CONTAINER_NAME"

# Add Python dev domains if requested
if [ "$PYTHON_DEV" = true ]; then
    DOMAINS+=("pypi.org" "files.pythonhosted.org")
fi

# Add GitHub domains if requested
if [ "$GITHUB" = true ]; then
    DOMAINS+=("github.com" "raw.githubusercontent.com" "objects.githubusercontent.com")
fi

# Add npm dev domains if requested
if [ "$NPM_DEV" = true ]; then
    DOMAINS+=("registry.npmjs.org")
fi

# Check if we have any domains to allow
if [ ${#DOMAINS[@]} -eq 0 ]; then
    echo "Usage: $0 [--domain <domain>]... [--python-dev] [--npm-dev] [--github]"
    echo ""
    echo "Examples:"
    echo "  $0 --domain api.github.com"
    echo "  $0 --python-dev"
    echo "  $0 --npm-dev"
    echo "  $0 --github"
    echo "  $0 --python-dev --github"
    exit 1
fi

# Show what will be allowed and ask for confirmation
echo ""
echo "📋 The following domains will be allowed network access:"
echo ""
for domain in "${DOMAINS[@]}"; do
    echo "  • $domain"
done
echo ""
read -p "Allow these domains? (y/n): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Cancelled"
    exit 0
fi

# Allow each domain
echo ""
echo "🔓 Allowing access to ${#DOMAINS[@]} domain(s)..."
for domain in "${DOMAINS[@]}"; do
    docker exec "$CONTAINER_NAME" /usr/local/bin/allow-domain.sh "$domain"
done

echo ""
echo "✅ All domains allowed successfully"
