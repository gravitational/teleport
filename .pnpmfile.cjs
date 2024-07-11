function afterAllResolved(lockfile, context) {
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