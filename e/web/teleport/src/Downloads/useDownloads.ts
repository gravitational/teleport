import { useState, useEffect } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import { compareSemVers } from 'teleport/lib/util';

import TeleportContextE from 'e-teleport/teleportContextE';

import { downloadObject } from 'e-teleport/services/downloads/downloads';

import type { Kind, OS, Release } from 'e-teleport/services/downloads';

export type State = ReturnType<typeof useDownloads>;

export const useDownloads = (ctx: TeleportContextE) => {
  const { attempt, run } = useAttempt('processing');
  const { attempt: licenseAttempt, run: runLicenseAttempt } = useAttempt('');

  const [releases, setReleases] = useState<Release[]>([]);
  const [availableVersions, setAvailableVersions] = useState<string[]>([]);

  const canDownloadLicense = ctx.storeUser.getLicenceAccess().read;
  const canDownloadReleaseAssets = ctx.storeUser.getDownloadAccess().list;

  useEffect(() => {
    if (canDownloadReleaseAssets) {
      run(() =>
        ctx.downloadsService.fetchReleases().then(res => {
          setReleases(res);
          setAvailableVersions(getAvailableVersions(res));
          if (res.length > 0) {
            setSelectedVersion(res[0].version);
          }
        })
      );
    }
  }, [canDownloadReleaseAssets, ctx.downloadsService, run]);

  // downloads filters
  const [selectedVersion, setSelectedVersion] = useState<string>(
    availableVersions.length > 0 ? availableVersions[0] : ''
  );
  const [selectedKind, setSelectedKind] = useState<Kind>('Teleport');
  const [selectedOS, setSelectedOS] = useState<OS>('Linux');

  function downloadLicense() {
    runLicenseAttempt(() =>
      ctx.downloadsService.fetchLicense().then(license => {
        if (license) {
          downloadObject('license.pem', license);
        }
      })
    );
  }

  return {
    attempt,
    releases,
    availableVersions,
    selectedVersion,
    setSelectedVersion,
    selectedKind,
    setSelectedKind,
    selectedOS,
    setSelectedOS,
    licenseAttempt,
    downloadLicense,
    canDownloadLicense,
    canDownloadReleaseAssets,
  };
};

const getAvailableVersions = (releases: Release[]): string[] => {
  return Array.from(
    new Set(
      releases
        .map(release => release.version)
        .sort(compareSemVers)
        .reverse()
    )
  );
};
