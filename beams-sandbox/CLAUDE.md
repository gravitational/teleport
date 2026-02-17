# Teleport Beams Sandbox - Network Restrictions

## What's Accessible

- ✅ DNS resolution works
- ✅ AI APIs: Anthropic, OpenAI, claude.ai
- ❌ Everything else is blocked (PyPI, npm, GitHub, external APIs, etc.)

## Proactively Request Network Access

**IMPORTANT:** Before attempting tasks that need network access, analyze requirements and request ALL needed domains upfront in a SINGLE command.

**WHERE TO RUN:** All `make allow` commands must be run **outside this sandbox** in a separate terminal on your laptop/workstation. You cannot modify network restrictions from inside the sandbox.

**CHECK FIRST:** Before requesting domains, read `/workspace/.beams/allowed-domains` to see what's already allowed. Only request domains NOT in this file. If the file doesn't exist, no domains have been allowed yet.

### Anticipate Requirements

Before starting work, identify what network access you'll need:
- Installing Python packages? → `--python-dev`
- Installing npm packages? → `--npm-dev`
- Cloning from GitHub? → `--github`
- Calling external APIs? → `--domain <api-domain>`
- Fetching data from URLs? → `--domain <url-domain>`

**Good Example:**
Task: "Write a Python script to get my IP using requests"

Your response:
```
Let me check what domains are already allowed...

[Read /workspace/.beams/allowed-domains - if api.ipify.org and pypi.org are there, skip the request]

To complete this task, I need network access. Please run in a separate terminal:

    make allow ARGS="--python-dev --domain api.ipify.org"

Once done, let me know and I'll proceed.
```

**Bad Example:**
❌ Try pip install → fails → ask for --python-dev → try API call → fails → ask for domain

### Allow Command Syntax

**Python + API (combined):**
```
make allow ARGS="--python-dev --domain api.github.com"
```

**Multiple domains:**
```
make allow ARGS="--domain api.github.com --domain raw.githubusercontent.com --python-dev"
```

**Single domain:**
```
make allow ARGS="--domain https://example.com"
```

### Common Patterns

- **Pip install + external API:** `--python-dev --domain <api-domain>`
- **npm install + API:** `--npm-dev --domain <api-domain>`
- **Git clone:** `--github`
- **Git clone + pip install:** `--github --python-dev`

## Example

```
❌ pip install requests fails
→ Ask user: "Please run: make allow-python-dev"
→ User runs it
✅ Retry: pip install requests
```

Keep requests minimal - only ask for access when needed.
