import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import { downloadObject } from 'shared/utils/download';
import { compareSemVers } from 'shared/utils/semVer';
import { wait } from 'shared/utils/wait';

import type { Kind, OS, Release } from 'e-teleport/services/downloads';
import { License } from 'e-teleport/services/downloads/types';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'teleport/config';

export type State = ReturnType<typeof useDownloads>;

export const useDownloads = (ctx: TeleportContextE) => {
  const [releases, setReleases] = useState<Release[]>([]);
  const { attempt, run } = useAttempt('processing');

  const [availableVersions, setAvailableVersions] = useState<string[]>([]);

  const [license, setLicense] = useState<License>();
  const { attempt: licenseAttempt, run: runLicenseAttempt } = useAttempt('');

  const canGenerateLicense = ctx.storeUser.getLicenceAccess().read;
  const canDownloadReleaseAssets =
    ctx.storeUser.getDownloadAccess().list || cfg.isCloud;

  useEffect(() => {
    if (canDownloadReleaseAssets) {
      const authVersion = ctx.storeUser.state.cluster.authVersion;
      run(() =>
        ctx.downloadsService.fetchReleases().then(res => {
          setReleases(res);
          let availableVersions = getAvailableVersions(res);
          if (ctx.isCloud) {
            // on cloud, only show versions compatible with the auth server
            availableVersions = removeIncompatibleVersions(
              authVersion,
              availableVersions
            );
          }
          setAvailableVersions(availableVersions);
          if (res.length > 0) {
            const latest = availableVersions[0];
            // on cloud, we use the auth server version as the default if available,
            // while for dashboards we just use the latest
            if (ctx.isCloud) {
              setSelectedVersion(
                availableVersions.includes(authVersion) ? authVersion : latest
              );
              return;
            }
            setSelectedVersion(latest);
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

/**
 * removeIncompatibleVersions returns a new array without the versions that
 * are not the same as 1 major less than the auth server.
 * @param currentVersion is the current auth server versions
 * @param versions an array of versions e.g.: ['10.0.1', '10.0.2']
 */
export function removeIncompatibleVersions(
  authVersion: string,
  versions: string[]
): string[] {
  return versions.filter(version => {
    const difference = majorDifference(authVersion, version);
    return difference === 0 || difference === 1;
  });
}

/**
 * Returns the difference between the major version of `a` and `b`.
 */
export function majorDifference(a: string, b: string): number | null {
  const splitA = a.split('.');
  const splitB = b.split('.');

  const majorA = parseInt(splitA[0]);
  const majorB = parseInt(splitB[0]);
  if (Number.isNaN(majorA) || Number.isNaN(majorB)) {
    return null;
  }
  return majorA - majorB;
}
