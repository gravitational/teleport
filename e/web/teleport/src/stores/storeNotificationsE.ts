import { subWeeks } from 'date-fns';
import { UserContext } from 'teleport/services/user';
import {
  StoreNotifications,
  Notification,
  NotificationKind,
} from 'teleport/stores/storeNotifications';

import { AccessList } from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

export class StoreNotificationsE extends StoreNotifications {
  setNotificationsForAccessListsRequiringReview(
    accessLists: AccessList[],
    userContext: UserContext
  ) {
    if (accessLists.length === 0) {
      this.setNotifications([]);
      return;
    }

    // Determine if context user is an admin or an owner of
    // the fetched access lists.
    // Members can also read access lists that they are
    // members of, but cannot modify or view them.

    // First check if this context's user is an admin.
    const { list, read, edit, create } = userContext.acl.accessList;
    const isAdmin = list && read && edit && create;
    if (!isAdmin) {
      // Check if this context's user is an owner.
      // We are only checking one of the access lists, because the
      // fetched list will only contain access lists that this
      // user is an owner to.
      const owners = accessLists[0].owners;
      if (!owners.some(o => o.name === userContext.username)) {
        return;
      }
    }

    // At this point, context user is either an admin or an owner.
    // Go through each access list and see which ones need
    // review in two weeks.

    const todayDate = new Date();
    const requiresReview = accessLists.filter(a =>
      accessListRequiresReview({
        todayDate,
        reviewDate: a.audit.nextDate,
      })
    );

    const notices: Notification[] = requiresReview.map(a => {
      return {
        id: a.id,
        date: a.audit.nextDate,
        item: {
          kind: NotificationKind.AccessList,
          resourceName: a.title,
          route: cfg.getAccessListManagementRoute(a.id),
        },
      };
    });

    this.updateNotificationsByKind(notices, NotificationKind.AccessList);
  }

  updateOrRemoveAccessListNotification(accessList: AccessList) {
    // Filter out possibly stale access list notice.
    const filtered = this.state.notifications.filter(
      n => n.item.kind === NotificationKind.AccessList && n.id !== accessList.id
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
          kind: NotificationKind.AccessList,
          resourceName: accessList.title,
          route: cfg.getAccessListManagementRoute(accessList.id),
        },
      });
    }

    this.updateNotificationsByKind(filtered, NotificationKind.AccessList);
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
