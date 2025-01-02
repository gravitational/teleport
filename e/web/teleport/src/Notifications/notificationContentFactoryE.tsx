import React from 'react';

import { ButtonSecondary } from 'design/Button';
import * as Icons from 'design/Icon';
import Text from 'design/Text';
import { useAsync } from 'shared/hooks/useAsync';
import Logger from 'shared/libs/logger';
import { pluralize } from 'shared/utils/text';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';
import {
  getLabelValue,
  NotificationContent,
  notificationContentFactory,
  QuickActionProps,
} from 'teleport/Notifications/notificationContentFactory';
import history from 'teleport/services/history';
import {
  LocalNotificationGroupedKind,
  LocalNotificationKind,
  NotificationSubKind,
  Notification as NotificationType,
} from 'teleport/services/notifications';
import session from 'teleport/services/websession';

const logger = Logger.create('Notifications');

/**
 notificationContentFactoryE produces the content for notifications for enterprise-only features.
 */
export function notificationContentFactoryE(
  notification: NotificationType
): NotificationContent {
  let notificationContent: NotificationContent;
  const { labels, subKind } = notification;

  // If it's an OSS notification, the OSS notification content factory can process it.
  notificationContent = notificationContentFactory(notification);
  if (notificationContent) {
    return notificationContent;
  }

  switch (subKind) {
    case NotificationSubKind.AccessRequestApproved: {
      const requestId = getLabelValue(labels, 'request-id');
      const assumableTime = getLabelValue(labels, 'assumable-time');
      const roles = getLabelValue(labels, 'roles').split(',');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'success',
        icon: Icons.Users,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
        QuickAction: ({ markAsClicked }) => (
          <AccessRequestAssumeButton
            requestId={requestId}
            assumableTime={assumableTime}
            markAsClicked={markAsClicked}
            roleCount={roles.length}
          />
        ),
      };
      break;
    }

    case NotificationSubKind.AccessRequestDenied: {
      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'failure',
        icon: Icons.Users,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
      };
      break;
    }

    case NotificationSubKind.AccessRequestPending: {
      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'informational',
        icon: Icons.UserList,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
      };
      break;
    }

    case LocalNotificationKind.AccessList:
      const redirectRoute = getLabelValue(labels, 'redirect-route');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'warning',
        icon: Icons.UserList,
        redirectRoute,
        hideDate: true,
      };
      break;

    case LocalNotificationGroupedKind.AccessListGrouping: {
      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'warning',
        icon: Icons.UserList,
        redirectRoute: cfg.getAccessListManagementRoute(null),
        hideDate: true,
      };
      break;
    }

    case NotificationSubKind.AccessRequestPromoted: {
      const requestId = getLabelValue(labels, 'request-id');

      notificationContent = {
        kind: 'redirect',
        title: notification.title,
        type: 'success',
        icon: Icons.ArrowFatLinesUp,
        redirectRoute: cfg.getAccessRequestRoute(requestId),
        QuickAction: ({ markAsClicked }) => (
          <ButtonSecondary
            onClick={(event: React.MouseEvent<HTMLButtonElement>) => {
              event.stopPropagation();
              markAsClicked();
              session.logout();
            }}
          >
            Log in again to gain access
          </ButtonSecondary>
        ),
      };
      break;
    }

    default:
      // If neither the OSS content factory nor this one was able to process it, there is a bug.
      logger.error(
        `Notification with title "${notification.title}" was unable to be processed.`
      );
      return null;
  }

  return notificationContent;
}

function AccessRequestAssumeButton({
  markAsClicked,
  requestId,
  assumableTime,
  roleCount,
}: QuickActionProps & {
  requestId: string;
  assumableTime: string;
  roleCount: number;
}) {
  const ctx = useTeleportE();

  const [assumeAttempt, assumeRequest] = useAsync(async () => {
    const req = await ctx.workflowService.fetchAccessRequest(requestId);
    const expires = await ctx.workflowService.applyPermission({ requestId });
    ctx.storeAccessRequests.addAssumed(req, expires);
    await markAsClicked();
    history.reload();
  });

  const isAssumable =
    !assumableTime || Date.now() >= new Date(assumableTime).getTime();

  const isAssumed = ctx.storeAccessRequests.isAssumed(requestId);

  const disabled =
    assumeAttempt.status === 'processing' || !isAssumable || isAssumed;

  return (
    <>
      <ButtonSecondary
        onClick={(event: React.MouseEvent<HTMLButtonElement>) => {
          event.stopPropagation();
          assumeRequest();
        }}
        disabled={disabled}
        title={!isAssumable ? 'This request is not assumable yet.' : ''}
      >
        {isAssumed ? 'Assumed' : `Assume ${pluralize(roleCount, 'Role')}`}
      </ButtonSecondary>
      {assumeAttempt.status === 'error' && (
        <Text typography="body3" color="error.main">
          Failed to assume {pluralize(roleCount, 'role')}:{' '}
          {assumeAttempt.statusText}
        </Text>
      )}
    </>
  );
}
