import React, { useEffect, useState } from 'react';

import SwitchBack from 'e-teleport/Banner/Switchback';
import { useBanner } from 'e-teleport/Banner/useBanner';
import cfg from 'e-teleport/config';
import { getEnterpriseFeatures } from 'e-teleport/features';
import {
  createErrorNotification,
  createSuccessNotification,
} from 'e-teleport/InviteCollaborators/common';
import {
  NotificationEntry,
  NotificationItem,
  Notifications,
} from 'e-teleport/InviteCollaborators/Notifications';
import TeleportEContext from 'e-teleport/teleportContextE';
import useTeleport from 'e-teleport/useTeleportE';
import { Questionnaire } from 'e-teleport/Welcome/Questionnaire/Questionnaire';
import { Main } from 'teleport/Main/Main';
import { storageService } from 'teleport/services/storageService';

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

  const isStripeManaged = cfg.oss.isStripeManaged;
  const requiresOnboardingSurvey = surveyUnanswered();
  // todo (michellescripts) rather than using isStripeManaged; surface Mode:Questionnaire
  const questionnaire =
    (isStripeManaged && requiresOnboardingSurvey && Questionnaire) || null;

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
      Questionnaire={questionnaire}
      inviteCollaboratorsFeedback={inviteCollaboratorsFeedback}
      topBarProps={
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
