const { env, platform } = require('process');
const fs = require('fs');
const { spawnSync } = require('child_process');
const isMac = platform === 'darwin';
const isWindows = platform === 'win32';

// The following checks make no sense when cross-building because they check the platform of the
// host and not the platform we're building for.
//
// However, at the moment we don't cross-build Connect and these checks protect us from undesired
// behavior.
//
// Also, we just want to make sure that those are explicitly set but they can be empty. That's why
// we check for undefined only and not for falsy values.
//
// Setting one of the env vars to an empty string is useful in environments where we don't intend to
// build a fully-fledged Connect version but rather want to just check that the packaging step is
// working, for example in CI.
if (
  isMac &&
  (env.CONNECT_TSH_APP_PATH === undefined) ===
    (env.CONNECT_TSH_BIN_PATH === undefined)
) {
  throw new Error(
    'You must provide CONNECT_TSH_APP_PATH xor CONNECT_TSH_BIN_PATH'
  );
}

if (!isMac && env.CONNECT_TSH_BIN_PATH === undefined) {
  throw new Error('You must provide CONNECT_TSH_BIN_PATH');
}

if (isWindows && env.CONNECT_WINTUN_DLL_PATH == undefined) {
  throw new Error('You must provide CONNECT_WINTUN_DLL_PATH');
}

// Holds tsh.app Info.plist during build. Used in afterPack.
let tshAppPlist;

// appId must be a reverse DNS string since it's also used as CFBundleURLName on macOS, see
// protocols.name below.
const appId = 'gravitational.teleport.connect';

/**
 * @type { import('electron-builder').Configuration }
 */
module.exports = {
  appId,
  asar: true,
  asarUnpack: '**\\*.{node,dll}',
  // TODO(ravicious): Migrate from custom notarize.js script to using the notarize field of the
  // mac target.
  afterSign: 'notarize.js',
  afterPack: packed => {
    // @electron-universal adds the `ElectronAsarIntegrity` key to every .plist
    // file it finds, causing signature verification to fail for tsh.app that gets
    // embedded in Teleport Connect. This causes the error "invalid Info.plist (plist
    // or signature have been modified)".
    // Workaround this by copying the tsp.app plist file before adding the key and
    // replace it after it is done.

    if (!env.CONNECT_TSH_APP_PATH) {
      // Not embedding tsh.app
      return;
    }

    const path = `${packed.appOutDir}/Teleport Connect.app/Contents/MacOS/tsh.app/Contents/Info.plist`;
    if (packed.appOutDir.endsWith('mac-universal-x64-temp')) {
      tshAppPlist = fs.readFileSync(path);
    }
    if (packed.appOutDir.endsWith('mac-universal')) {
      if (!tshAppPlist) {
        throw new Error(
          'Failed to copy tsh.app Info.plist file from the x64 build. Check if the path "mac-universal-x64-temp" was not changed by electron-builder.'
        );
      }
      fs.writeFileSync(path, tshAppPlist);
    }
  },
  files: ['build/app'],
  protocols: [
    {
      // name ultimately becomes CFBundleURLName which is the URL identifier. [1] Apple recommends
      // to set it to a reverse DNS string. [2]
      //
      // [1] https://developer.apple.com/documentation/bundleresources/information_property_list/cfbundleurltypes/cfbundleurlname
      // [2] https://developer.apple.com/documentation/xcode/defining-a-custom-url-scheme-for-your-app#Register-your-URL-scheme
      name: appId,
      schemes: ['teleport'],
      // Not much documentation is available on the role attribute. It ultimately gets mapped to
      // CFBundleTypeRole in Info.plist.
      //
      // It seems that this field is largely related to how macOS thinks of "documents". Since Connect
      // doesn't let you really edit anything and we won't be passing any docs, let's just set it to
      // 'Viewer'.
      //
      // https://cocoadev.github.io/CFBundleTypeRole/
      role: 'Viewer',
    },
  ],
  mac: {
    target: 'dmg',
    category: 'public.app-category.developer-tools',
    type: 'distribution',
    // TODO(ravicious): Migrate from custom notarize.js script to using the notarize field of the
    // mac target.
    notarize: false,
    hardenedRuntime: true,
    gatekeeperAssess: false,
    // If CONNECT_TSH_APP_PATH is provided, we assume that tsh.app is already signed.
    signIgnore: env.CONNECT_TSH_APP_PATH && ['tsh.app'],
    icon: 'build_resources/icon-mac.png',
    // x64ArchFiles is for x64 and universal files (lipo tool should skip them)
    x64ArchFiles: 'Contents/MacOS/tsh.app/Contents/MacOS/tsh',
    // On macOS, helper apps (such as tsh.app) should be under Contents/MacOS, hence using
    // `extraFiles` instead of `extraResources`.
    // https://developer.apple.com/documentation/bundleresources/placing_content_in_a_bundle
    // https://developer.apple.com/forums/thread/128166
    extraFiles: [
      // CONNECT_TSH_APP_PATH is for environments where we want to copy over the whole signed
      // version of tsh.app for Touch ID support.
      env.CONNECT_TSH_APP_PATH && {
        from: env.CONNECT_TSH_APP_PATH,
        to: './MacOS/tsh.app',
      },
      // CONNECT_TSH_BIN_PATH is for environments where we just need a regular tsh binary. We still
      // copy it to the same location that it would be at in a real tsh.app to avoid conditional
      // logic elsewhere.
      env.CONNECT_TSH_BIN_PATH && {
        from: env.CONNECT_TSH_BIN_PATH,
        to: './MacOS/tsh.app/Contents/MacOS/tsh',
      },
    ].filter(Boolean),
  },
  dmg: {
    artifactName: '${productName}-${version}-${arch}.${ext}',
    // Turn off blockmaps since we don't support automatic updates.
    // https://github.com/electron-userland/electron-builder/issues/2900#issuecomment-730571696
    writeUpdateInfo: false,
    contents: [
      {
        x: 130,
        y: 220,
      },
      {
        x: 410,
        y: 220,
        type: 'link',
        path: '/Applications',
      },
    ],
  },
  win: {
    target: ['nsis'],
    // The algorithm passed here is not used, it only prevents the signing function from being called twice for each file.
    // https://github.com/electron-userland/electron-builder/issues/3995#issuecomment-505725704
    signingHashAlgorithms: ['sha256'],
    sign: customSign => {
      if (process.env.CI !== 'true') {
        console.warn('Not running in CI pipeline: signing will be skipped');
        return;
      }

      spawnSync(
        'powershell',
        [
          '-noprofile',
          '-executionpolicy',
          'bypass',
          '-c',
          "$ProgressPreference = 'SilentlyContinue'; " +
            "$ErrorActionPreference = 'Stop'; " +
            '. ../../../build.assets/windows/build.ps1; ' +
            `Invoke-SignBinary -UnsignedBinaryPath "${customSign.path}"`,
        ],
        { stdio: 'inherit' }
      );
    },
    artifactName: '${productName} Setup-${version}.${ext}',
    icon: 'build_resources/icon-win.ico',
    extraResources: [
      env.CONNECT_TSH_BIN_PATH && {
        from: env.CONNECT_TSH_BIN_PATH,
        to: './bin/tsh.exe',
      },
      env.CONNECT_WINTUN_DLL_PATH && {
        from: env.CONNECT_WINTUN_DLL_PATH,
        to: './bin/wintun.dll',
      },
    ].filter(Boolean),
  },
  nsis: {
    // Turn off blockmaps since we don't support automatic updates.
    // https://github.com/electron-userland/electron-builder/issues/2900#issuecomment-730571696
    differentialPackage: false,
  },
  rpm: {
    artifactName: '${name}-${version}.${arch}.${ext}',
    afterInstall: 'build_resources/linux/after-install.tpl',
    afterRemove: 'build_resources/linux/after-remove.tpl',
    // --rpm-rpmbuild-define "_build_id_links none" fixes the problem with not being able to install
    // Connect's rpm next to other Electron apps.
    // https://github.com/gravitational/teleport/issues/18859
    fpm: ['--rpm-rpmbuild-define', '_build_id_links none'],
  },
  deb: {
    artifactName: '${name}_${version}_${arch}.${ext}',
    afterInstall: 'build_resources/linux/after-install.tpl',
    afterRemove: 'build_resources/linux/after-remove.tpl',
  },
  linux: {
    target: ['tar.gz', 'rpm', 'deb'],
    artifactName: '${name}-${version}-${arch}.${ext}', // tar.gz
    category: 'Development',
    icon: 'build_resources/icon-linux',
    extraResources: [
      env.CONNECT_TSH_BIN_PATH && {
        from: env.CONNECT_TSH_BIN_PATH,
        to: './bin/tsh',
      },
      {
        from: 'build_resources/linux/apparmor-profile',
        to: './apparmor-profile',
      },
    ].filter(Boolean),
  },
  directories: {
    buildResources: 'build_resources',
    output: 'build/release',
  },
};
