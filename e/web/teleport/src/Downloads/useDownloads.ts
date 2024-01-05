import { useState, useEffect } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import { compareSemVers } from 'shared/utils/semVer';

import { wait } from 'shared/utils/wait';

import TeleportContextE from 'e-teleport/teleportContextE';

import { downloadObject } from 'e-teleport/services/downloads/downloads';

import { License } from 'e-teleport/services/downloads/types';

import type { Kind, OS, Release } from 'e-teleport/services/downloads';

export type State = ReturnType<typeof useDownloads>;

export const useDownloads = (ctx: TeleportContextE) => {
  const [releases, setReleases] = useState<Release[]>([]);
  const { attempt, run } = useAttempt('processing');

  const [availableVersions, setAvailableVersions] = useState<string[]>([]);

  const [license, setLicense] = useState<License>();
  const { attempt: licenseAttempt, run: runLicenseAttempt } = useAttempt('');

  const canGenerateLicense = ctx.storeUser.getLicenceAccess().read;
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

  // Note: we generate the license as soon as we render the UI, because we want
  // to show the expiration date.
  useEffect(() => {
    if (canGenerateLicense) {
      runLicenseAttempt(() =>
        ctx.downloadsService.fetchLicense().then(setLicense)
      );
    }
  }, [canGenerateLicense, ctx.downloadsService, runLicenseAttempt]);

  // downloads filters
  const [selectedVersion, setSelectedVersion] = useState<string>(
    availableVersions.length > 0 ? availableVersions[0] : ''
  );
  const [selectedKind, setSelectedKind] = useState<Kind>('Teleport');
  const [selectedOS, setSelectedOS] = useState<OS>('Linux');

  const [showSaveLicenseDialog, setShowSaveLicenseDialog] = useState(false);

  async function generateLicense() {
    // Fetch a fresh license, make sure that we have a latency of at least 1
    // second. The purpose of this ensured latency is to provide visual clue
    // to the user that the license is being actually generated on the fly and
    // unique every time.
    const success = await runLicenseAttempt(() =>
      Promise.all([fetchLicense(), wait(1000)])
    );
    setShowSaveLicenseDialog(success);
  }

  async function fetchLicense(): Promise<License> {
    const newLicense = await ctx.downloadsService.fetchLicense();
    setLicense(newLicense);
    return newLicense;
  }

  function saveLicense() {
    downloadObject('license.pem', license.pem);
  }

  function closeSaveLicenseDialog() {
    setShowSaveLicenseDialog(false);
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
    generateLicense,
    saveLicense,
    canGenerateLicense,
    showSaveLicenseDialog,
    closeSaveLicenseDialog,
    canDownloadReleaseAssets,
    license,
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
