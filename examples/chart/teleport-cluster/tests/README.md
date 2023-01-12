## Unit tests for Helm charts

Helm chart unit tests run here using the [helm-unittest](https://github.com/quintush/helm-unittest/) Helm plugin.

*Note: there are multiple forks for the helm-unittest plugin.
They are not compatible and don't provide the same featureset (e.g. including templates from sub-directories).
Our tests rely on features and bugfixes that are only available on the quintush fork
(which seems to be the most maintained at the time of writing)*

If you get a snapshot error during your testing, you should verify that your changes intended to alter the output, then run
this command from the root of your Teleport checkout to update the snapshots:

```bash
make -C build.assets test-helm-update-snapshots
```

After this, re-run the tests to make sure everything is fine:

```bash
make -C build.assets test-helm
```

Commit the updated snapshots along with your changes.
