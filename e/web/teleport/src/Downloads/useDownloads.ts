import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import { downloadObject } from 'shared/utils/download';
import { wait } from 'shared/utils/wait';

import { License } from 'e-teleport/services/downloads/types';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'teleport/config';

export type State = ReturnType<typeof useDownloads>;

export const useDownloads = (ctx: TeleportContextE) => {
  const [license, setLicense] = useState<License>();
  const { attempt: licenseAttempt, run: runLicenseAttempt } = useAttempt('');

  const canGenerateLicense = ctx.storeUser.getLicenceAccess().read;
  const canDownloadReleaseAssets =
    ctx.storeUser.getDownloadAccess().list || cfg.isCloud;
  const authVersion = ctx.storeUser.state.cluster.authVersion;

  // Note: we generate the license as soon as we render the UI, because we want
  // to show the expiration date.
  useEffect(() => {
    if (canGenerateLicense) {
      runLicenseAttempt(() =>
        ctx.downloadsService.fetchLicense().then(setLicense)
      );
    }
  }, [canGenerateLicense, ctx.downloadsService, runLicenseAttempt]);

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
    canDownloadReleaseAssets,
    authVersion,
    canGenerateLicense,
    generateLicense,
    saveLicense,
    licenseAttempt,
    showSaveLicenseDialog,
    closeSaveLicenseDialog,
    license,
  };
};
