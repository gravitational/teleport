#!/bin/sh

# If this build was triggered from a tag on the teleport repo,
# then assume we can use the same tag for the webapps repo.
if [ -n "$DRONE_TAG" ]
then
    echo "$DRONE_TAG"
    exit 0
fi

# If this build is on one of the teleport release branches,
# map to the equivalent release branch in webapps.
#
# branch/v10 ==> teleport-v10
if echo "$DRONE_TARGET_BRANCH" | grep '^branch/' >/dev/null;
then
    TRIMMED=$(echo $DRONE_TARGET_BRANCH | cut -c8-)
    echo "teleport-$TRIMMED"
    exit 0
fi

# Otherwise, use master.
echo "master"
