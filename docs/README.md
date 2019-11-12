# Teleport Docs

Teleport docs are built using [mkdocs](http://www.mkdocs.org/) and hosted
as a bunch of static files in S3.

Look at `build.sh` script to see how it works.

## To Publish New Version

* Update [build.sh](build.sh).
* Update theme/scripts.html to add a new version to the docVersions array of versions
* Create a new YAML file, like `5.5.1.yaml` if you are releasing version 5.5.1

## Deploying

Teleport docs are published using a private `web` repository.
See `web/README.md` for more info.

## Running Locally

We recommend using Docker to run and build the docs. 

`make run-docs` will run the docs and setup a [livereload server](https://chrome.google.com/webstore/detail/livereload/jnihajbhpnppcggbcgedagnkighmdlei?hl=en) for easy previewing of changes.  

`make docs` will build the docs, so they are ready to ship to production. 


## Tools used to build the Docs

Teleport Docs are made with MkDocs and a few markdown extensions, First time users will need to install MkDocs https://www.mkdocs.org/#installation. 

To run the latest version of the docs on [http://127.0.0.1:8000](http://127.0.0.1:8000/quickstart):

```bash
$ ./run.sh
```

To run a specific version of the docs:

```bash
$ mkdocs serve --config-file 1.3.yaml
```

## Checking for bad links

Install [milv (Markdown Internal Link Validator)](https://github.com/magicmatatjahu/milv) using `go get -u -v github.com/magicmatatjahu/milv`.

Change to the appropriate base directory:

`$ cd docs/4.1`

Run `milv`:

`$ milv`

Make sure that you fix any broken links or errors before committing them.

If there is a genuine false positive or case where `milv` is failing to parse a link correctly but it does work (and you have validated this using `make run-docs` or `mkdocs serve`) then you can add a whitelist under the appropriate section in `milv.config.yaml` and commit this back to the repo.
