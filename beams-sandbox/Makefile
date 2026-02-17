.PHONY: build run clean help

IMAGE_NAME := beams-sandbox
WORKSPACE_DIR := $(shell pwd)/workspace

help: ## Show this help message
	@echo "Beams Sandbox - Local Testing Environment"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the beams sandbox Docker image
	@docker build -q -t $(IMAGE_NAME) . > /dev/null

run: build ## Build and run the beams sandbox interactively
	@if [ -z "$$CLAUDE_CODE_OAUTH_TOKEN" ]; then \
		echo "⚠️  Warning: CLAUDE_CODE_OAUTH_TOKEN is not set"; \
		echo "Claude Code will not be authenticated in the sandbox"; \
	fi
	@docker rm -f beams-sandbox-instance 2>/dev/null || true
	@docker run -it \
		--name beams-sandbox-instance \
		--cap-add=NET_ADMIN \
		-e CLAUDE_CODE_OAUTH_TOKEN="$$CLAUDE_CODE_OAUTH_TOKEN" \
		--entrypoint /usr/local/bin/restrict-network.sh \
		$(IMAGE_NAME) \
		/bin/bash
	@docker rm -f beams-sandbox-instance 2>/dev/null || true

stop: ## Stop the sandbox
	@docker rm -f beams-sandbox-instance 2>/dev/null || true

clean: ## Remove the Docker image
	@docker rmi $(IMAGE_NAME) || true

allow: ## Allow domains (usage: make allow ARGS="--domain abc.com --domain def.com --python-dev")
	@./allow.sh $(ARGS)

run-claude-yolo-test: build ## Run Claude Code non-interactively with auto-approve and network restrictions
	@if [ -z "$$CLAUDE_CODE_OAUTH_TOKEN" ]; then \
		echo "❌ Error: CLAUDE_CODE_OAUTH_TOKEN is not set"; \
		exit 1; \
	fi
	docker run --rm \
		--cap-add=NET_ADMIN \
		-e CLAUDE_CODE_OAUTH_TOKEN="$$CLAUDE_CODE_OAUTH_TOKEN" \
		$(IMAGE_NAME) \
		/usr/local/bin/restrict-network.sh claude -p "write a python app that gets my ip via requests and run it" --dangerously-skip-permissions
