# This script prints a list of dynamic resource kinds as a partial to include in
# the documentation.
set -eo pipefail
DESTINATION="../../../docs/pages/includes/resource_kinds.mdx";
SOURCE_DIR="build.assets/tooling/cmd";

if (pwd | grep -vq "${SOURCE_DIR}$"); then
  echo "You must run this script from build.assets/tooling/cmd in your teleport clone."
  exit 1
fi

echo "{/* Generated file. Do not edit. */}" > ${DESTINATION};
echo "{/* To regenerate this file, navigate to ${SOURCE_DIR} and execute list_resource_kinds.sh */}" >> ${DESTINATION};
echo "" >> ${DESTINATION};
echo '```text' >> ${DESTINATION};
# An assignment of a resource kind looks like this:
#
#	KindRole = "role"
#
# List all kind names based on this assignment convention.
grep -E "^\s+Kind[A-Za-z]+\s+=" ../../../api/types/constants.go | sed -E 's/.*"([a-z_]+)".*/\1/' | sort >> ${DESTINATION};
echo '```text' >> ${DESTINATION};
