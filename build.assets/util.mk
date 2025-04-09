# This file contains a bunch of "utilities" for using make.

# Define some chars as variables as they can be hard to use literally
# in places
empty =
space = $(empty) $(empty)
comma = ,
hash = \#
# Two newlines in nl definition as the last is removed by make.
define nl


endef

# Detect if running on github. If not, assume local. This var should be
# expanded to support detection of other CI systems.
CI_SYSTEM = $(if $(GITHUB_ACTIONS),github,local)

# LOG_GROUP_START and LOG_GROUP_END can be called at the start and end of make
# recipes that produce a lot of output or would otherwise benefit from being
# grouped. On GitHub, the group is placed into an collapsable section. For local
# runs, start/end markers are printed.
#
# Use in a recipe:
# target:
#         $(call LOG_GROUP_START,Build the thing)
#         ...
#         $(call LOG_GROUP_END,Build the thing)
#
# If the message is left out of the call, the default message is
# "make <target>".

LOG_GROUP_START = $(LOG_GROUP_START_$(CI_SYSTEM))
LOG_GROUP_END = $(LOG_GROUP_END_$(CI_SYSTEM))

# Implement LOG_GROUP_START/END for all supported CI systems and "local".
log_group_label = $(or $(1),make $@)
LOG_GROUP_START_github = @echo '::group::$(log_group_label)'
LOG_GROUP_END_github = @echo '::endgroup::'
LOG_GROUP_START_local = @echo '$(hash)$(hash) Start: $(log_group_label)'
LOG_GROUP_END_local = @echo '$(hash)$(hash) End: $(log_group_label)'


