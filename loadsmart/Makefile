# Prepares a custom Loadmart Teleport release to trigger the CircleCI pipeline and push the image to the Loadsmart ECR registry.
.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is not set. Usage: make ls-release VERSION=17.5.1"; \
		exit 1; \
	fi
	@./release.sh $(VERSION)
