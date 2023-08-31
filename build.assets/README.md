# Dockerized Teleport Build

This directory is used to produce a containerized production Teleport build.
No need to have Golang. Only Docker is required.

It is a part of Gravitational CI/CD pipeline. To build Teleport type:

```
make
```

# Safely updating build box Dockerfiles

The build box images are used in Drone pipelines and GitHub Actions. The resulting image is pushed
to Amazon ECR and ghcr.io. This means that to safely introduce changes to Dockerfiles, those changes
should be split into two stages:

1. First you open a PR which updates a Dockerfile and get the PR merged.
2. Once it's merged, Drone is going to pick it up, build a new build box image and push it to Amazon
   ECR.
3. Then you can open another PR which starts using the new build box image.

# DynamoDB static binary docker build

The static binary will be built along with all nodejs assets inside the container.
From the root directory of the source checkout run:

```
docker build -f build.assets/Dockerfile.dynamodb -t teleportbuilder .
```

Then you can upload the result to an S3 bucket for release.

```
docker run -it -e AWS_ACL=public-read -e S3_BUCKET=my-teleport-releases -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY teleportbuilder
```

Or simply copy the binary out of the image using a volume (it will be copied to current directory/build/teleport).

```
docker run -v $(pwd)/build:/builds -it teleportbuilder cp /gopath/src/github.com/gravitational/teleport/teleport.tgz /builds
```

# OS package repo migrations

An OS package repo migration is semi-manually publishing specific releases to the new APT and YUM repos. This is required in several situations:

- A customer requests that we add an older version to the repos
- We add another OS package repo (for example APK)
- A OS package promotion fails (for example https://drone.platform.teleport.sh/gravitational/teleport/14666/1/3), requires a PR to fix, and we don't want to cut another minor version

Multiple migrations can be performed at once. To run a migration do the following:

1. Clone https://github.com/gravitational/teleport.git.
2. Change to the directory the repo was cloned to.
3. Create a new branch from master.
4. Add the Teleport versions you wish to migration as demonstrated here: https://github.com/gravitational/teleport/commit/151a2f489e3116fc7ce8f55e056529361d3233a6#diff-2e3a64c97d186491e06fb2c7ead081b7ace2b67c4a4d974a563daf7c117a2c50.
5. Set the `migrationBranch` variable to the name of the branch you created in (3) as demonstrated here: https://github.com/gravitational/teleport/commit/151a2f489e3116fc7ce8f55e056529361d3233a6#diff-2e3a64c97d186491e06fb2c7ead081b7ace2b67c4a4d974a563daf7c117a2c50.
6. Get your Drone credentials from here: https://drone.platform.teleport.sh/account.
7. Export your drone credentials as shown under "Example CLI Usage" on the Drone account page
8. Open a new terminal.
9. Run `tsh apps login drone` and follow any prompts.
10. Run `tsh proxy app drone` and copy the printed socket. This should look something like `127.0.0.1:60982`
11. Switch back to your previous terminal.
12. Run `export DRONE_SERVER=http://{host:port}`, replacing `{host:port}` with the data you copied in (10)
13. Run `make dronegen`
14. Commit the two changed files and push/publish the branch
15. Open a PR merging your changes into master via https://github.com/gravitational/teleport/compare
16. Under the "checks" section, click "details" on the check labeled "continuous-integration/drone/push"
17. Once the pipelines complete, comment out the versions you added and blank out the `migrationBranch` string set in (4, 5) as demonstrated here: https://github.com/gravitational/teleport/pull/15531/commits/9095880560cfe6c93e491e39a7604b1faf72c600#diff-2e3a64c97d186491e06fb2c7ead081b7ace2b67c4a4d974a563daf7c117a2c50
18. Run `make dronegen`
19. Commit and push the changes.
20. Merge the PR and backport if required.
