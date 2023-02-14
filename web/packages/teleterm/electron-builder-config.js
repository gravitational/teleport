const {
  createDefaultConfig,
} = require('@gravitational/build/electron-builder-config');

module.exports = createDefaultConfig({ buildResourcesPath: 'build_resources' });
