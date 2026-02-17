# Teleport Beams Sandbox
# A sandboxed environment for AI agents with restricted network access.
FROM node:23-slim

ENV DEBIAN_FRONTEND=noninteractive

# -----------------------------------------------------------------------------
# System tools
# -----------------------------------------------------------------------------
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    vim \
    jq \
    git \
    ca-certificates \
    iptables \
    dnsutils \
    && rm -rf /var/lib/apt/lists/*

# -----------------------------------------------------------------------------
# Python
# -----------------------------------------------------------------------------
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    && ln -s /usr/bin/python3 /usr/bin/python \
    && rm -rf /var/lib/apt/lists/*

# Allow pip install without restrictions (sandbox environment)
RUN rm -f /usr/lib/python3*/EXTERNALLY-MANAGED
ENV PIP_ROOT_USER_ACTION=ignore
ENV PIP_RETRIES=1

# -----------------------------------------------------------------------------
# Node.js / npm
# -----------------------------------------------------------------------------
# node:23-slim includes Node.js 23 which supports TypeScript natively (node file.ts)
ENV NPM_CONFIG_FETCH_RETRIES=0

# -----------------------------------------------------------------------------
# Claude Code
# -----------------------------------------------------------------------------
RUN npm install -g @anthropic-ai/claude-code

# Skip browser-based auth (token is passed via CLAUDE_CODE_OAUTH_TOKEN env var)
ENV CLAUDE_CODE_SKIP_BROWSER=true

# -----------------------------------------------------------------------------
# User
# -----------------------------------------------------------------------------
# Claude Code requires a non-root user for --dangerously-skip-permissions
RUN useradd -m -s /bin/bash beams

# pip and npm install to user-local directories
ENV PATH="/home/beams/.local/bin:/home/beams/.npm-global/bin:$PATH"
ENV NPM_CONFIG_PREFIX=/home/beams/.npm-global

# Skip onboarding and pre-trust /workspace
RUN mkdir -p /home/beams/.claude && \
    echo '{"hasCompletedOnboarding":true,"projects":{"/workspace":{"hasTrustDialogAccepted":true}}}' > /home/beams/.claude.json && \
    chown -R beams:beams /home/beams/.claude

# -----------------------------------------------------------------------------
# Workspace
# -----------------------------------------------------------------------------
WORKDIR /workspace
RUN chown beams:beams /workspace

# CLAUDE.md provides Claude Code with instructions about network restrictions
RUN mkdir -p /workspace/.claude
COPY CLAUDE.md /workspace/.claude/CLAUDE.md
RUN chown -R beams:beams /workspace/.claude

# -----------------------------------------------------------------------------
# Network restriction scripts (used by Makefile on host)
# -----------------------------------------------------------------------------
COPY restrict-network.sh /usr/local/bin/restrict-network.sh
COPY allow-domain.sh /usr/local/bin/allow-domain.sh
RUN chmod +x /usr/local/bin/restrict-network.sh /usr/local/bin/allow-domain.sh

# -----------------------------------------------------------------------------
# Shell
# -----------------------------------------------------------------------------
COPY motd /etc/motd
RUN echo 'cat /etc/motd' >> /home/beams/.bashrc && \
    chown beams:beams /home/beams/.bashrc

CMD ["/bin/bash"]
