import { useEffect } from 'react';

import { useToastNotifications } from 'shared/components/ToastNotification';

import SwitchBack from 'e-teleport/Banner/Switchback';
import cfg from 'e-teleport/config';
import { getEnterpriseFeatures } from 'e-teleport/features';
import {
  createErrorNotification,
  createSuccessNotification,
} from 'e-teleport/InviteCollaborators/common';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
import useTeleport from 'e-teleport/useTeleportE';
import { Main } from 'teleport/Main/Main';
import { storageService } from 'teleport/services/storageService';

import { BblpLogo } from './bblpLogo';
import { BeamsLogo } from './beamsLogo/BeamsLogo';
import { CscoLogo } from './cscoLogo';
import { McLogo } from './mcLogo';

export function MainE() {
  const ctx = useTeleport();
  const toastNotification = useToastNotifications();

  useEffect(() => {
    const userInvites = storageService.getCloudUserInvites();
    if (userInvites) {
      ctx.cloudService
        .sendTeleportInvite(userInvites)
        .then(() => {
          toastNotification.add(createSuccessNotification(userInvites));
        })
        .catch(err => {
          toastNotification.add(createErrorNotification(err));
        })
        .finally(() => storageService.clearCloudUserInvites());
    }
  }, []);

  const customBanners = [];
  if (ctx.storeAccessRequests.getSessionExpiry()) {
    customBanners.push(
      <SwitchBack key="access-request-banner" data-testid="banner" />
    );
  }

  const CustomLogos: Record<string, () => React.ReactElement> = {
    bblp: BblpLogo,
    csco: CscoLogo,
    mc: McLogo,
  };

  let customLogo = cfg.oss.customTheme
    ? CustomLogos[cfg.oss.customTheme]
    : undefined;

  if (!customLogo && cfg.oss.beamsUi) {
    customLogo = BeamsLogo;
  }

  return (
    <AccessGraphDemoProvider>
      <Main
        features={getEnterpriseFeatures()}
        customBanners={customBanners}
        CustomLogo={customLogo}
      />
    </AccessGraphDemoProvider>
  );
}
