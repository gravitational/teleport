set -e -o pipefail

# Intended to be run at the root of gravitational/teleport

# $1: Failure message
fail () {
  msg="FAILURE: $1"
  FAILURES="$FAILURES\n$msg"
}

# Use newlines for the IFS. We need to iterate through a list of filenames
# and, for each file name, a list of expected error substrings.
IFS="
"
FILES=$(jq -r 'keys | join("\n")' < .github/vale-tests/tests.json);
for f in $(echo -e "$FILES"); do
    ERRORS=$(vale --no-exit --output line ".github/vale-tests/$f")
    echo "$ERRORS"
    EXPECTED=$(jq --arg file "$f" -r '.[$file] | join("\n")' < .github/vale-tests/tests.json)
    if [ -z "$EXPECTED" ] && [ -n "$ERRORS" ]; then
    	fail "Found unexpected errors in $f"
    	continue
    fi

    for x in $(echo -e "$EXPECTED"); do
      echo "$ERRORS" | grep -q "$x" || fail "No error found with substring: \"$x\" in $f"
    done
done

if [ -z "$FAILURES" ]; then
  echo -e "\nAll vale tests passed."
else
  echo -e "$FAILURES"
  exit 1
fi



