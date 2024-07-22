function afterAllResolved(lockfile, context) {
  // The pnpm lockfile contains entries for all workspaces.
  // This means that if the `e` repo is not cloned, we will get a different
  // lockfile.
  // The ugly workaround is to add an entry for e/web/teleport workspace manually.
  // We pass an empty object because `e/web/teleport` package.json doesn't have
  // any dependencies.
  if (!lockfile.importers['e/web/teleport']) {
    context.log(`Workspace 'e/web/teleport' is not present, patching lockfile.`);
    lockfile.importers['e/web/teleport'] = {};
  }
  return lockfile;
}

module.exports = {
  hooks: {
    afterAllResolved,
  },
};
