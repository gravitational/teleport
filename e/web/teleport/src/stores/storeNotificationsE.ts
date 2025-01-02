import { subWeeks } from 'date-fns';

import cfg from 'e-teleport/config';
import { AccessList } from 'e-teleport/services/accessmanagement';
import { LocalNotificationKind } from 'teleport/services/notifications';
import { UserContext } from 'teleport/services/user';
import {
  Notification,
  StoreNotifications,
} from 'teleport/stores/storeNotifications';

export class StoreNotificationsE extends StoreNotifications {
  setNotificationsForAccessListsRequiringReview(
    accessLists: AccessList[],
    userContext: UserContext
  ) {
    if (accessLists.length === 0) {
      this.setNotifications([]);
      return;
    }

    const todayDate = new Date();
    // Go through each access list and see which ones need
    // review in two weeks and if requires review, check if
    // the currently logged in user needs to be notified.
    const requiresReview = accessLists.filter(
      a =>
        accessListRequiresReview({
          todayDate,
          reviewDate: a.audit.nextDate,
        }) && shouldNotifyForReview(a, userContext)
    );

    const notices: Notification[] = requiresReview.map(a => {
      return {
        id: a.id,
        date: a.audit.nextDate,
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: a.title,
          route: cfg.getAccessListManagementRoute(a.id),
        },
      };
    });

    this.updateNotificationsByKind(notices, LocalNotificationKind.AccessList);
  }

  /**
   * Updates or removes an access list from existing notifications.
   * If user is not a owner or have admin privileges, then this
   * function will do nothing.
   */
  updateOrRemoveAccessListNotification(
    accessList: AccessList,
    userContext: UserContext
  ) {
    if (!shouldNotifyForReview(accessList, userContext)) {
      return;
    }

    // Filter out possibly stale access list notice.
    const filtered = this.state.notifications.filter(
      n =>
        n.item.kind === LocalNotificationKind.AccessList &&
        n.id !== accessList.id
    );
    // Add the latest access list notice if requires review.
    if (
      accessListRequiresReview({
        todayDate: new Date(),
        reviewDate: accessList.audit.nextDate,
      })
    ) {
      filtered.push({
        id: accessList.id,
        date: accessList.audit.nextDate,
        item: {
          kind: LocalNotificationKind.AccessList,
          resourceName: accessList.title,
          route: cfg.getAccessListManagementRoute(accessList.id),
        },
      });
    }

    this.updateNotificationsByKind(filtered, LocalNotificationKind.AccessList);
  }
}

export function accessListRequiresReview({
  todayDate,
  reviewDate,
}: {
  todayDate: Date;
  reviewDate: Date;
}) {
  if (!reviewDate) {
    return false;
  }

  return todayDate >= subWeeks(reviewDate, 2);
}

/**
 * Determines if context user (the currently logged in user) should be
 * notified that this access list requires attention.
 *
 * Only returns true for the following:
 *   - User is found in the owners list
 *   - User has RBAC priviledges == admin
 */
function shouldNotifyForReview(
  accessList: AccessList,
  userContext: UserContext
) {
  const { list, read, edit, create } = userContext.acl.accessList;
  const isAdmin = list && read && edit && create;
  if (isAdmin) {
    return true;
  }

  const loggedInUser = userContext.username;

  // If loggedInUser is not a owner, then they are just a member.
  return accessList.owners.some(o => o.name === loggedInUser);
}
