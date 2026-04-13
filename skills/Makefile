# CLAUDE_SKILLS_DIR is where skills for Claude Code are.
CLAUDE_SKILLS_DIR ?= ~/.claude/skills

# SKILLS_DIR is the directory containing all skill subdirectories.
SKILLS_DIR := $(CURDIR)

# package-skills packages all agent skills in .zip archives that can be uploaded
# to Claude Desktop app.
#
# It's needed because Claude Desktop allows either a single SKILL.md file or
# the skill archive, you can't upload a skill with its additional metadata as
# a directory.
.PHONY: package-skills
package-skills: clean-skill-packages
	@for dir in $(SKILLS_DIR)/*/; do \
		echo "Zipping $$dir..."; \
		name=$$(basename "$$dir"); \
		zip -r "$$name.zip" "$$name"; \
	done

# clean-skill-packages removes all .zip archives from the skills directory.
.PHONY: clean-skill-packages
clean-skill-packages:
	find $(SKILLS_DIR) -name "*.zip" -delete

# install-skills creates symlinks in ~/.claude/skills pointing to each skill
# directory.
#
# Useful for local skills development with terminal Claude Code.
.PHONY: install-skills
install-skills:
	@mkdir -p $(CLAUDE_SKILLS_DIR)
	@for dir in $(SKILLS_DIR)/*/; do \
		name=$$(basename "$$dir"); \
		target=$(CLAUDE_SKILLS_DIR)/$$name; \
		src=$$dir; \
		echo "Creating symlink: $$target -> $$src"; \
		ln -sf "$$src" "$$target"; \
	done

# uninstall-skills removes symlinks from ~/.claude/skills that point into this
# directory.
.PHONY: uninstall-skills
uninstall-skills:
	@for dir in $(SKILLS_DIR)/*/; do \
		name=$$(basename "$$dir"); \
		target=$(CLAUDE_SKILLS_DIR)/$$name; \
		if [ -L "$$target" ]; then \
			echo "Removing symlink: $$target"; \
			rm "$$target"; \
		fi; \
	done

# require-pre-commit checks that pre-commit is installed.
.PHONY: require-pre-commit
require-pre-commit:
	@which pre-commit 1>/dev/null 2>&1 || (echo 'pre-commit is not installed. For installation instructions see https://pre-commit.com/#install'; exit 1)

# enable-pre-commit installs the pre-commit hooks for skill validation.
.PHONY: enable-pre-commit
enable-pre-commit: require-pre-commit
	pre-commit install -c skills/.pre-commit-config.yaml

# disable-pre-commit uninstalls the pre-commit hooks for skill validation.
.PHONY: disable-pre-commit
disable-pre-commit: require-pre-commit
	pre-commit uninstall -c skills/.pre-commit-config.yaml
