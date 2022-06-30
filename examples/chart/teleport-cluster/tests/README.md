## Unit tests for Helm charts

Helm chart unit tests run here using the [helm3-unittest](https://github.com/vbehar/helm3-unittest) Helm plugin.

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
