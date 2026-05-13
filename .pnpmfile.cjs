function afterAllResolved(lockfile, context) {
  // The pnpm lockfile contains entries for all workspaces.
  // This means that if the `e` repo is not cloned, we will get a different
  // lockfile.
  // The ugly workaround is to add entries for enterprise workspaces manually.
  if (!lockfile.importers['e/web/teleport']) {
    context.log(`Workspace 'e/web/teleport' is not present, patching lockfile.`);
    lockfile.importers['e/web/teleport'] = {};
  }
  if (!lockfile.importers['e/e2e']) {
    context.log(`Workspace 'e/e2e' is not present, patching lockfile.`);
    // Mirror @playwright/test from the OSS e2e workspace so the injected
    // entry tracks whatever version pnpm resolved there, without duplicating
    // the version string here.
    const ossPlaywright =
      lockfile.importers['e2e']?.devDependencies?.['@playwright/test'];
    lockfile.importers['e/e2e'] = {
      dependencies: {
        '@gravitational/e2e': 'link:../../e2e',
        '@playwright/test': ossPlaywright.version,
      },
      specifiers: {
        '@gravitational/e2e': 'workspace:*',
        '@playwright/test': ossPlaywright.specifier,
      },
    };
  }
  return lockfile;
}

module.exports = {
  hooks: {
    afterAllResolved,
  },
};