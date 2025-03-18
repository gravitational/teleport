import React, { useEffect, useState } from 'react';

import SwitchBack from 'e-teleport/Banner/Switchback';
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
import { Main } from 'teleport/Main/Main';
import { storageService } from 'teleport/services/storageService';

import { BblpLogo } from './bblpLogo';

export function MainE() {
  const ctx = useTeleport();

  const customBanners = [];
  if (ctx.storeAccessRequests.getSessionExpiry()) {
    customBanners.push(
      <SwitchBack key="access-request-banner" data-testid="banner" />
    );
  }

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
      customBanners={customBanners}
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
