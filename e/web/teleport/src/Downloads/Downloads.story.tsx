import { DownloadsView } from './Downloads';
import { State } from './useDownloads';

export default {
  title: 'TeleportE/Downloads',
};

export const Loaded = () => <DownloadsView {...props} />;

export const GeneratingLicense = () => (
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

export const GeneratedLicense = () => (
  <DownloadsView {...props} showSaveLicenseDialog />
);

export const NoPermissionReleases = () => (
  <DownloadsView {...props} canDownloadReleaseAssets={false} />
);

export const NoPermissionLicense = () => (
  <DownloadsView {...props} canGenerateLicense={false} />
);

const props: State = {
  canDownloadReleaseAssets: true,
  authVersion: '10.3.1',
  canGenerateLicense: true,
  generateLicense: async () => {},
  saveLicense: () => {},
  licenseAttempt: {
    status: 'success',
    statusText: '',
  },
  showSaveLicenseDialog: false,
  closeSaveLicenseDialog: () => {},
  license: {
    pem: 'pem',
    expiry: new Date('2025-12-17T12:00:00'),
  },
};
