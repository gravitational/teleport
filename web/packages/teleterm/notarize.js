const { notarize } = require('@electron/notarize');

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

  if (!process.env.TEAMID) {
    console.warn(
      'missing $TEAMID: notarization will be skipped. Run `make release-connect` instead'
    );
    return;
  }

  const appName = context.packager.appInfo.productFilename;
  const appBundleId = context.packager.appInfo.macBundleIdentifier;

  console.log('notarize.js: Notarizing in progress.');

  return await notarize({
    appBundleId,
    appPath: `${appOutDir}/${appName}.app`,
    appleId: process.env.APPLE_USERNAME,
    appleIdPassword: process.env.APPLE_PASSWORD,
    tool: 'notarytool',
    teamId: process.env.TEAMID,
  });
};
