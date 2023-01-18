import React from 'react';

import { DownloadsView } from './Downloads';

import { State } from './useDownloads';

export default {
  title: 'TeleportE/Downloads',
};

export const LoadedLinux = () => <DownloadsView {...props} />;
export const LoadedWindows = () => (
  <DownloadsView {...props} selectedOS="Windows" />
);
export const LoadedMacOS = () => (
  <DownloadsView {...props} selectedOS="macOS" />
);

export const LoadingReleases = () => (
  <DownloadsView
    {...props}
    attempt={{
      status: 'processing',
      statusText: '',
    }}
  />
);

export const FailedReleases = () => (
  <DownloadsView
    {...props}
    attempt={{
      status: 'failed',
      statusText: 'failed loading releases',
    }}
  />
);

export const LoadingLicense = () => (
  <DownloadsView
    {...props}
    licenseAttempt={{
      status: 'processing',
      statusText: '',
    }}
  />
);

export const FailedLicense = () => (
  <DownloadsView
    {...props}
    licenseAttempt={{
      status: 'failed',
      statusText: 'failed loading license',
    }}
  />
);

export const NoPermissionReleases = () => (
  <DownloadsView {...props} canDownloadReleaseAssets={false} />
);

export const NoPermissionLicense = () => (
  <DownloadsView {...props} canDownloadLicense={false} />
);

const props: State = {
  canDownloadReleaseAssets: true,
  canDownloadLicense: true,
  attempt: {
    status: 'success',
    statusText: '',
  },
  availableVersions: ['10.3.1', '8.3.19'],
  downloadLicense: () => {},
  licenseAttempt: {
    status: 'success',
    statusText: '',
  },
  releases: [
    {
      version: '10.3.1',
      assets: [
        {
          name: 'teleport-connect_10.3.1_amd64.deb',
          os: 'Linux',
          kind: 'Teleport Connect',
          displaySize: '79.8 MB',
          description: 'Teleport Connect 64-bit DEB',
          sha256:
            '3bdaa71c986cb2478228509e8f1c2471957e0b34ccaa7b7703e061c8ee318512',
          url: 'https://cdn.cloud.gravitational.io/teleport-connect_10.3.1_amd64.deb',
        },
        {
          name: 'teleport-ent-10.3.1-1.arm64.rpm',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '100.0 MB',
          description: 'Linux ARM64/ARMv8 (64-bit) RPM',
          sha256:
            '01c9d580508a894bb28617bd28c7210ad954a91467cad75f003433b6f9881aed',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent-10.3.1-1.arm64.rpm',
        },
        {
          name: 'teleport-ent-10.3.1-1.arm.rpm',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '100.1 MB',
          description: 'Linux ARMv7 (32-bit) RPM',
          sha256:
            '0fc28a13ea69789374b0e0b5714849d7a26d3692bf17487f342bf37b8eeb6019',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent-10.3.1-1.arm.rpm',
        },
        {
          name: 'teleport-ent_10.3.1_amd64.deb',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '114.0 MB',
          description: 'Linux 64-bit DEB',
          sha256:
            'a1c5a2dbbb7fada5f543665a2b0a98608cc9e30e429680cc73623b1ac50325eb',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent_10.3.1_amd64.deb',
        },
        {
          name: 'teleport-ent_10.3.1_i386.deb',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '105.5 MB',
          description: 'Linux 32-bit DEB',
          sha256:
            '485cacf55a8542f0d7ddf3484af2b21cd46c4d7d1024329a0a700235ab729e0f',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent_10.3.1_i386.deb',
        },
        {
          name: 'teleport-ent-v10.3.1-darwin-amd64-bin.tar.gz',
          os: 'macOS',
          kind: 'Teleport',
          displaySize: '116.3 MB',
          description: 'MacOS Intel',
          sha256:
            '5e350e64dae9e81381690638857b7a44c2c7c4e0859289a573327d3085b651de',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent-v10.3.1-darwin-amd64-bin.tar.gz',
        },
        {
          name: 'teleport-ent-v10.3.1-linux-386-bin.tar.gz',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '105.9 MB',
          description: 'Linux 32-bit',
          sha256:
            'a1eb1bde033e689fdcf95aad16c0da222ba7208c1506b3e14e9d5c1567bba465',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent-v10.3.1-linux-386-bin.tar.gz',
        },
        {
          name: 'teleport-ent-v10.3.1-linux-amd64-bin.tar.gz',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '114.3 MB',
          description: 'Linux 64-bit',
          sha256:
            '0e50c8fced9d7ca22af74f364b3826ce40bb27f24c26605617546273f52248c3',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent-v10.3.1-linux-amd64-bin.tar.gz',
        },
        {
          name: 'tsh-10.3.1.pkg',
          os: 'macOS',
          kind: 'tsh client',
          displaySize: '25.5 MB',
          description: 'MacOS Intel .pkg installer (tsh client only)',
          sha256:
            'de39ddff121f662b3872f2e4884953e6e0c972326a431feda857a6c8997b8414',
          url: 'https://cdn.cloud.gravitational.io/tsh-10.3.1.pkg',
        },
      ],
    },
    {
      version: '8.3.19',
      assets: [
        {
          name: 'teleport-ent-8.3.19-1.arm64.rpm',
          os: 'Linux',
          kind: 'Teleport',
          displaySize: '60.9 MB',
          description: 'Linux ARM64/ARMv8 (64-bit) RPM',
          sha256:
            '780724ed2736d021c7fb4cecb4aa436e935c71c3c55486e81f1f273ed1b935a1',
          url: 'https://cdn.cloud.gravitational.io/teleport-ent-8.3.19-1.arm64.rpm',
        },
      ],
    },
  ],
  selectedKind: 'Teleport',
  selectedOS: 'Linux',
  selectedVersion: '10.3.1',
  setSelectedKind: () => {},
  setSelectedOS: () => {},
  setSelectedVersion: () => {},
};
