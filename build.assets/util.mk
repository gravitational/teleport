# This file contains a bunch of "utilities" for using make.

# Detect if running on github. If not, assume local. This var should be
# expanded to support detection of other CI systems.
CI_SYSTEM = $(if $(GITHUB_ACTIONS),github,local)

# LOG_GROUP_START and LOG_GROUP_END can be called at the start and end of make
# recipes that produce a lot of output or would otherwise benefit from being
# grouped. On GitHub, the group is placed into an collapsable section. For
# local runs, start/end markers are printed.
#
# Use in a recipe:
#
# target:
#         $(call LOG_GROUP_START)
#         ...
#         $(call LOG_GROUP_END)
#
# The default title is "make <target>" which can be overridden by providing
# the title as a parameter to the $(call)s: $(call LOG_GROUP_START,Boogying)
# and $(call LOG_GROUP_END).

LOG_GROUP_START = $(LOG_GROUP_START_$(CI_SYSTEM))
LOG_GROUP_END = $(LOG_GROUP_END_$(CI_SYSTEM))

# Use the provided title if provided or default to "make <target>".
log_group_title = $(or $(1),make $@)

# GitHub has a group marker.
# https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions#grouping-log-lines
LOG_GROUP_START_github = @echo '::group::$(log_group_title)'
LOG_GROUP_END_github = @echo '::endgroup::'

# Use a markdown syntax for local, even if the rest of the output is not
# markdown. The markers are reasonably searchable.
LOG_GROUP_START_local = @echo '\#\# Start: $(log_group_title)'
LOG_GROUP_END_local = @echo '---'
