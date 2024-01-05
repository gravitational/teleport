import React from 'react';
import { FeatureBox } from 'teleport/components/Layout';

import useTeleportE from 'e-teleport/useTeleportE';

import { useDownloads, State } from './useDownloads';
import { License } from './License/License';
import { TeleportReleases } from './TeleportReleases/TeleportReleases';

export function Downloads() {
  const ctx = useTeleportE();

  const state = useDownloads(ctx);
  return <DownloadsView {...state} />;
}

export const GETTING_STARTED_LINK =
  'https://goteleport.com/docs/deploy-a-cluster/teleport-enterprise/getting-started/?scope=enterprise';
export const LICENSE_FILE_GUIDE_LINK =
  'https://goteleport.com/docs/deploy-a-cluster/teleport-enterprise/license/';

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
      <License
        canGenerateLicense={canGenerateLicense}
        showSaveLicenseDialog={showSaveLicenseDialog}
        closeSaveLicenseDialog={closeSaveLicenseDialog}
        generateLicense={generateLicense}
        saveLicense={saveLicense}
        licenseAttempt={licenseAttempt}
        expiry={license?.expiry}
      />
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
