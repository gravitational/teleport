import { addWeeks, subWeeks } from 'date-fns';
import { Access, makeUserContext } from 'teleport/services/user';

import {
  AccessList,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';

import { StoreNotificationsE } from './storeNotificationsE';

const baseUserContext = makeUserContext({
  cluster: {
    name: 'cluster-1',
    lastConnected: '2020-09-26T17:30:23.512876876Z',
  },
});

test('no notices are set if not an owner and or not an admin', async () => {
  const store = new StoreNotificationsE();

  expect(store.getNotifications()).toStrictEqual([]);

  store.setNotificationsForAccessListsRequiringReview(mocks, {
    ...baseUserContext,
    acl: {
      ...baseUserContext.acl,
      accessList: noAccess, // not an admin
    },
    username: 'not-a-owner',
  });

  // filtered and sorted
  expect(store.getNotifications()).toHaveLength(0);
  expect(store.getNotifications()).toStrictEqual([]);
});

test('as an admin, set and filter access lists due within two weeks', async () => {
  const store = new StoreNotificationsE();

  expect(store.getNotifications()).toStrictEqual([]);

  store.setNotificationsForAccessListsRequiringReview(mocks, {
    ...baseUserContext,
    acl: {
      ...baseUserContext.acl,
      accessList: hasAccess,
    },
    username: 'does-not-matter',
  });

  // filtered and sorted
  expect(store.getNotifications()).toHaveLength(4);
  expect(store.getNotifications()).toStrictEqual([
    {
      item: {
        kind: 'access-list',
        resourceName: pastDue.title,
        route: `/web/accesslists/${pastDue.id}`,
      },
      id: pastDue.id,
      date: pastDue.audit.nextDate,
    },
    {
      item: {
        kind: 'access-list',
        resourceName: dueToday.title,
        route: `/web/accesslists/${dueToday.id}`,
      },
      id: dueToday.id,
      date: dueToday.audit.nextDate,
    },
    {
      item: {
        kind: 'access-list',
        resourceName: dueInOneWeek.title,
        route: `/web/accesslists/${dueInOneWeek.id}`,
      },
      id: dueInOneWeek.id,
      date: dueInOneWeek.audit.nextDate,
    },
    {
      item: {
        kind: 'access-list',
        resourceName: dueInTwoWeeks.title,
        route: `/web/accesslists/${dueInTwoWeeks.id}`,
      },
      id: dueInTwoWeeks.id,
      date: dueInTwoWeeks.audit.nextDate,
    },
  ]);
});

test('as an owner, set and filter access lists due within two weeks', async () => {
  const store = new StoreNotificationsE();
  expect(store.getNotifications()).toStrictEqual([]);

  const userContext = {
    ...baseUserContext,
    acl: {
      ...baseUserContext.acl,
      accessList: noAccess, // not an admin
    },
    username: 'llama',
  };

  // Should do nothing, even though its due for a review since
  // user is not an owner of the access list.
  store.updateOrRemoveAccessListNotification(
    dueTodayButNotAnOwner,
    userContext
  );
  expect(store.getNotifications()).toStrictEqual([]);

  // Test for reviews by being an owner.
  store.setNotificationsForAccessListsRequiringReview(
    mocksForLlama,
    userContext
  );

  // filtered and sorted
  expect(store.getNotifications()).toHaveLength(2);
  expect(store.getNotifications()).toStrictEqual([
    {
      item: {
        kind: 'access-list',
        resourceName: dueInOneWeek.title,
        route: `/web/accesslists/${dueInOneWeek.id}`,
      },
      id: dueInOneWeek.id,
      date: dueInOneWeek.audit.nextDate,
    },
    {
      item: {
        kind: 'access-list',
        resourceName: dueInTwoWeeks.title,
        route: `/web/accesslists/${dueInTwoWeeks.id}`,
      },
      id: dueInTwoWeeks.id,
      date: dueInTwoWeeks.audit.nextDate,
    },
  ]);
});

test('update an access list notification (due date changed, but still due within two weeks)', async () => {
  const store = new StoreNotificationsE();

  expect(store.getNotifications()).toStrictEqual([]);

  const userContext = {
    ...baseUserContext,
    acl: {
      ...baseUserContext.acl,
      accessList: hasAccess,
    },
    username: 'does-not-matter',
  };
  store.setNotificationsForAccessListsRequiringReview(mocks, userContext);
  expect(store.getNotifications()).toHaveLength(4);

  // Update an existing notification "dueToday".
  store.updateOrRemoveAccessListNotification(
    {
      ...dueToday,
      audit: { ...dueToday.audit, nextDate: addWeeks(new Date(), 2) },
    },
    userContext
  );

  const notices = store.getNotifications();
  expect(notices).toHaveLength(4);

  const updatedDueToday = notices.find(n => n.id === dueToday.id);
  expect(updatedDueToday).toBeTruthy();
  expect(updatedDueToday.date.getTime()).toBeGreaterThan(
    dueToday.audit.nextDate.getTime()
  );
});

test('remove an existing notification (no longer due in two weeks)', async () => {
  const store = new StoreNotificationsE();

  expect(store.getNotifications()).toStrictEqual([]);

  const userContext = {
    ...baseUserContext,
    acl: {
      ...baseUserContext.acl,
      accessList: hasAccess,
    },
    username: 'does-not-matter',
  };
  store.setNotificationsForAccessListsRequiringReview(mocks, userContext);
  expect(store.getNotifications()).toHaveLength(4);

  // Update an existing notification "dueToday".
  store.updateOrRemoveAccessListNotification(
    {
      ...dueToday,
      audit: { ...dueToday.audit, nextDate: addWeeks(new Date(), 6) },
    },
    userContext
  );

  const notices = store.getNotifications();
  expect(notices).toHaveLength(3);

  expect(notices.find(n => n.id === dueToday.id)).toBeFalsy();
});

const dueToday: AccessList = {
  id: 'due-today',
  title: 'due today',
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
    },
    nextDate: new Date(),
  },
  grants: { roles: [], traits: {} },
  ownerGrants: { roles: [], traits: {} },
  ownershipRequires: { roles: [], traits: {} },
  owners: [{ name: 'alpaca' }],
};

const dueTodayButNotAnOwner: AccessList = {
  ...dueToday,
  id: 'due-today-but-not-an-owner',
  owners: [{ name: 'random' }],
};

const pastDue: AccessList = {
  id: 'past-due',
  title: 'past-due',
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
    },
    nextDate: subWeeks(new Date(), 2),
  },
  grants: { roles: [], traits: {} },
  ownerGrants: { roles: [], traits: {} },
  ownershipRequires: { roles: [], traits: {} },
  owners: [{ name: 'apple' }],
};

const dueInOneWeek: AccessList = {
  id: 'due-in-one-week',
  title: 'due in one week',
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
    },
    nextDate: addWeeks(new Date(), 1),
  },
  grants: { roles: [], traits: {} },
  ownerGrants: { roles: [], traits: {} },
  ownershipRequires: { roles: [], traits: {} },
  owners: [{ name: 'alpaca' }, { name: 'llama' }],
};

const dueInTwoWeeks: AccessList = {
  id: 'due-in-two-weeks',
  title: 'due in two weeks',
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
    },
    nextDate: addWeeks(new Date(), 2),
  },
  grants: { roles: [], traits: {} },
  ownerGrants: { roles: [], traits: {} },
  ownershipRequires: { roles: [], traits: {} },
  owners: [{ name: 'alpaca' }, { name: 'llama' }],
};

const mocks: AccessList[] = [
  dueInTwoWeeks,
  {
    id: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
    title: 'Interns',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      },
      nextDate: addWeeks(new Date(), 3),
    }, // not require review
    grants: { roles: [], traits: {} },
    ownerGrants: { roles: [], traits: {} },
    ownershipRequires: { roles: [], traits: {} },
    owners: [{ name: 'alpaca' }, { name: 'llama' }],
  },
  dueToday,
  pastDue,
  {
    id: '47dadc5f-4840-5ad1-bcb6-ed63ded98937',
    title: 'Design Team',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      },
      nextDate: addWeeks(new Date(), 4),
    }, // not require review
    grants: { roles: [], traits: {} },
    ownerGrants: { roles: [], traits: {} },
    ownershipRequires: { roles: [], traits: {} },
    owners: [],
  },
  dueInOneWeek,
];

const mocksForLlama: AccessList[] = [
  dueTodayButNotAnOwner,
  {
    id: '47dadc5f-4840-5ad1-bcb6-ed63ded98937',
    title: 'Design Team',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      },
      nextDate: addWeeks(new Date(), 4),
    }, // not require review
    grants: { roles: [], traits: {} },
    ownerGrants: { roles: [], traits: {} },
    ownershipRequires: { roles: [], traits: {} },
    owners: [{ name: 'llama' }],
  },
  dueInTwoWeeks,
  {
    id: '47dadc5f-4840-5ad1-bcb6-ed63ded98937',
    title: 'Design Team',
    audit: {
      recurrence: {
        frequency: ReviewFrequency.OneYear,
        dayOfMonth: ReviewDayOfMonth.LastDayOfMonth,
      },
      nextDate: addWeeks(new Date(), 3),
    }, // not require review
    grants: { roles: [], traits: {} },
    ownerGrants: { roles: [], traits: {} },
    ownershipRequires: { roles: [], traits: {} },
    owners: [{ name: 'llama' }],
  },
  dueInOneWeek,
];

const hasAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

const noAccess: Access = {
  list: false,
  read: false,
  edit: false,
  create: false,
  remove: false,
};
