# If this build was triggered from a tag on the teleport repo,
# then assume we can use the same tag for the webapps repo.
if (Test-Path Env:DRONE_TAG) {
    Write-Output $Env:DRONE_TAG
    exit 0
}

# If this build is on one of the teleport release branches,
# map to the equivalent release branch in webapps.
#
# branch/v10 ==> teleport-v10
if ($Env:DRONE_TARGET_BRANCH -match '^branch/(.*)$') {
    Write-Output "teleport-$($Matches[1])"
    exit 0
}

# Otherwise, use master.
Write-Output "master"