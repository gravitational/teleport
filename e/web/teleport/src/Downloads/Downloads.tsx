import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { License } from './License/License';
import { TeleportReleases } from './TeleportReleases/TeleportReleases';
import { State, useDownloads } from './useDownloads';

export function Downloads() {
  const ctx = useTeleportE();

  const state = useDownloads(ctx);
  return <DownloadsView {...state} />;
}

export const LICENSE_FILE_GUIDE_LINK =
  'https://goteleport.com/docs/admin-guides/deploy-a-cluster/license/';

export const DownloadsView = ({
  attempt,
  releases,
  availableVersions,
  selectedVersion,
  setSelectedVersion,
  selectedKind,
  setSelectedKind,
  selectedOS,
  setSelectedOS,
  generateLicense,
  saveLicense,
  licenseAttempt,
  canGenerateLicense,
  showSaveLicenseDialog,
  closeSaveLicenseDialog,
  canDownloadReleaseAssets,
  license,
}: State) => {
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Downloads</FeatureHeaderTitle>
      </FeatureHeader>

      {!cfg.oss.isCloud && (
        <License
          canGenerateLicense={canGenerateLicense}
          showSaveLicenseDialog={showSaveLicenseDialog}
          closeSaveLicenseDialog={closeSaveLicenseDialog}
          generateLicense={generateLicense}
          saveLicense={saveLicense}
          licenseAttempt={licenseAttempt}
          expiry={license?.expiry}
        />
      )}
      <TeleportReleases
        canDownloadReleaseAssets={canDownloadReleaseAssets}
        releases={releases}
        attempt={attempt}
        availableVersions={availableVersions}
        selectedVersion={selectedVersion}
        setSelectedVersion={setSelectedVersion}
        selectedOS={selectedOS}
        setSelectedOS={setSelectedOS}
        selectedKind={selectedKind}
        setSelectedKind={setSelectedKind}
      />
    </FeatureBox>
  );
};
