import React, { ReactNode, useMemo, useState, useEffect } from 'react';

import { Main } from 'teleport/Main/Main';

import { storageService } from 'teleport/services/storageService';

import { useBanner } from 'e-teleport/Banner/useBanner';
import useTeleport from 'e-teleport/useTeleportE';
import TeleportEContext from 'e-teleport/teleportContextE';
import SwitchBack from 'e-teleport/Banner/Switchback';
import { getEnterpriseFeatures } from 'e-teleport/features';
import cfg from 'e-teleport/config';
import { StripeLoader } from 'e-teleport/Billing/StripeLoader/StripeLoader';
import { BillingInformation } from 'e-teleport/services/cloud';
import { UsageBasedUpgrade } from 'e-teleport/Banner/UsageBasedUpgrade/UsageBasedUpgrade';
import { Questionnaire } from 'e-teleport/Welcome/Questionnaire/Questionnaire';
import {
  Notifications,
  NotificationEntry,
  NotificationItem,
} from 'e-teleport/InviteCollaborators/Notifications';
import {
  createErrorNotification,
  createSuccessNotification,
} from 'e-teleport/InviteCollaborators/common';

import { BblpLogo } from './bblpLogo';

export function MainE() {
  const ctx = useTeleport();
  const { license } = useBanner();

  // TODO(hatch): In the near future the license warning will come over the same cluster
  //              alerts endpoint and this check along with the `useBanner` call can be
  //              removed.
  const initialAlerts = [];
  if (license) {
    initialAlerts.push({
      kind: 'license-warning',
      version: 'v1',
      metadata: {
        name: 'license-warning',
        labels: {},
      },
      expires: '',
      spec: {
        severity: 10,
        message: license.text,
        created: '',
      },
    });
  }

  const customBanners = [];
  if (ctx.storeAccessRequests.getSessionExpiry()) {
    customBanners.push(
      <SwitchBack key="access-request-banner" data-testid="banner" />
    );
  }

  const isTeam = cfg.oss.isTeam;

  const teamUpgradeBanner = useMemo(() => {
    if (!isTeam) {
      return;
    }

    return (
      <StripeLoader
        key={'stripe-upgrade'}
        dataSource={(CloudService): Promise<BillingInformation> =>
          CloudService.fetchBillingInformation()
        }
        render={(data: BillingInformation, reload: () => void): ReactNode => (
          <UsageBasedUpgrade billingInfo={data} reload={reload} />
        )}
      />
    );
  }, [isTeam]);

  const billingBanners = [];
  if (isTeam) {
    billingBanners.push(teamUpgradeBanner);
  }

  const requiresOnboardingSurvey = surveyUnanswered();
  const questionnaire =
    (isTeam && requiresOnboardingSurvey && Questionnaire) || null;

  const CustomLogos = {
    bblp: BblpLogo,
  };

  const [inviteNotificationCount, setInviteNotificationCount] =
    useState<number>(0);
  const [inviteNotifications, setInviteNotifications] = useState<
    NotificationItem[]
  >([]);
  const inviteCollaboratorsFeedback = InviteCollaboratorsFeedback({
    ctx,
    notifications: inviteNotifications,
    setNotifications: setInviteNotifications,
    notificationCount: inviteNotificationCount,
    setNotificationCount: setInviteNotificationCount,
  });

  return (
    <Main
      features={getEnterpriseFeatures()}
      initialAlerts={initialAlerts}
      customBanners={customBanners}
      billingBanners={billingBanners}
      Questionnaire={questionnaire}
      inviteCollaboratorsFeedback={inviteCollaboratorsFeedback}
      navigationProps={
        cfg.oss.customTheme && {
          CustomLogo: CustomLogos[cfg.oss.customTheme],
          showPoweredByLogo: !!CustomLogos[cfg.oss.customTheme],
        }
      }
    />
  );
}

type InviteCollaboratorsFeedbackProps = {
  ctx: TeleportEContext;
  notifications: NotificationItem[];
  setNotifications: (
    notifications: React.SetStateAction<NotificationItem[]>
  ) => void;
  notificationCount: number;
  setNotificationCount: (
    notificationCount: React.SetStateAction<number>
  ) => void;
};

function InviteCollaboratorsFeedback({
  ctx,
  notifications,
  setNotifications,
  notificationCount,
  setNotificationCount,
}: InviteCollaboratorsFeedbackProps): React.ReactElement {
  function addNotification(item: NotificationEntry) {
    setNotifications([
      {
        ...item,
        id: notificationCount.toString(),
        dismissAfterMs: item.dismissAfterMs,
      },
      ...notifications,
    ]);
    setNotificationCount(notificationCount + 1);
  }

  function dismissNotification(id: string) {
    setNotifications(notifications.filter(i => i.id != id));
  }

  useEffect(() => {
    const userInvites = storageService.getCloudUserInvites();
    if (userInvites) {
      ctx.cloudService
        .sendTeleportInvite(userInvites)
        .then(() => addNotification(createSuccessNotification(userInvites)))
        .catch(err => addNotification(createErrorNotification(err)))
        .finally(() => storageService.clearCloudUserInvites());
    }
  }, []);

  return <Notifications items={notifications} dismiss={dismissNotification} />;
}

// SurveyUnanswered checks both the user preferences and the survey
// since survey data is moved into preferences on login, this means a user may have just filled
// out the survey but the results are not yet in preferences.
const surveyUnanswered = (): boolean => {
  const onboardPreferences = storageService.getOnboardUserPreference();

  if (
    onboardPreferences &&
    onboardPreferences.preferredResources &&
    onboardPreferences.preferredResources.length > 0
  ) {
    return false;
  }

  const survey = storageService.getOnboardSurvey();
  return !(
    survey &&
    survey.clusterResources &&
    survey.clusterResources.length > 0
  );
};
