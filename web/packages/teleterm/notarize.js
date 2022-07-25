const { notarize } = require('electron-notarize');

const package = require('./package.json');

exports.default = async function notarizing(context) {
  const { electronPlatformName, appOutDir } = context;
  if (electronPlatformName !== 'darwin') {
    return;
  }

  if (!process.env.APPLE_USERNAME || !process.env.APPLE_PASSWORD) {
    console.warn(
      'missing $APPLE_USERNAME and/or $APPLE_PASSWORD: notarization will be skipped'
    );
    return;
  }

  const appName = context.packager.appInfo.productFilename;

  return await notarize({
    appBundleId: package.build.appId,
    appPath: `${appOutDir}/${appName}.app`,
    appleId: process.env.APPLE_USERNAME,
    appleIdPassword: process.env.APPLE_PASSWORD,
  });
};
