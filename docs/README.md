# Teleport Docs

Teleport docs are built using [mkdocs](http://www.mkdocs.org/) and hosted
as a bunch of static files in S3.

Look at `build.sh` script to see how it works.

## To Publish New Version

* Update [build.sh](build.sh).
* Update [index.html](index.html) to redirect to the latest version automatically.
* Create a new YAML file, like `5.5.1.yaml` if you are releasing version 5.5.1

## Deploying

Teleport docs are published using a private `web` repository.
See `web/README.md` for more info.
