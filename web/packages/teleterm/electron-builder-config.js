/**
 * @type { import('electron-builder').Configuration }
 */
module.exports = {
  appId: 'gravitational.teleport.connect',
  asar: true,
  asarUnpack: '**\\*.{node,dll}',
  afterSign: 'notarize.js',
  files: ['build/app/dist'],
  mac: {
    target: 'dmg',
    category: 'public.app-category.developer-tools',
    type: 'distribution',
    hardenedRuntime: true,
    gatekeeperAssess: false,
    icon: 'assets/icon-mac.png',
    extraResources: [
      {
        from: '../../../teleport/build/tsh',
        to: './bin/tsh',
      },
    ],
  },
  dmg: {
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
    artifactName: '${productName} Setup-${version}.${ext}',
    icon: 'assets/icon-win.ico',
    extraResources: [
      {
        from: '../../../teleport/build/tsh.exe',
        to: './bin/tsh.exe',
      },
    ],
  },
  linux: {
    target: ['tar.gz', 'rpm', 'deb'],
    category: 'Development',
    icon: 'assets/icon-linux',
    extraResources: [
      {
        from: '../../../teleport/build/tsh',
        to: './bin/tsh',
      },
    ],
  },
  directories: {
    buildResources: 'assets',
    output: 'build/release',
  },
  extraResources: ['./assets/**'],
};
