#!/bin/bash
set -eo pipefail

USED_IMG=$(mktemp /tmp/docs-img-used.XXXX);
ALL_IMG=$(mktemp /tmp/docs-img-all.XXXX);
USED_PARTIALS=$(mktemp /tmp/docs-partials-used.XXXX);
ALL_PARTIALS=$(mktemp /tmp/docs-partials-all.XXXX);

find docs/img -type f | sed "s|^docs/||" | sort > "$ALL_IMG";
find docs/pages/includes -type f | sort > "$ALL_PARTIALS";

grep -hERo "img/[A-Za-z0-9@\._\/-]+" docs/pages | sort >"$USED_IMG";
grep -hERo "docs/pages/includes/[A-Za-z0-9@\._\/-]+" docs/pages | sort >"$USED_PARTIALS";

UNUSED_IMG=$(comm -23 "$ALL_IMG" "$USED_IMG");
UNUSED_PARTIALS=$(comm -23 "$ALL_PARTIALS" "$USED_PARTIALS");

rm "$USED_IMG"
rm "$ALL_IMG"
rm "$USED_PARTIALS"
rm "$ALL_PARTIALS"

# Restore the "docs" path segment so a user can pipe this into rm or similar.
FULL_UNUSED_IMG=$(echo "$UNUSED_IMG" | sed -E "s|(img/[A-Za-z0-9@\._\/-]+)|docs/\1|");

# Exit with no error if there are only empty lines in the output.
printf "%s\n" $FULL_UNUSED_IMG $UNUSED_PARTIALS | grep -v "^$" || exit 0;
