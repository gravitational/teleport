import { createContext, useContext, useEffect, useRef, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import { compareByString } from 'teleport/lib/util';
import { ApiError } from 'teleport/services/api/parseError';

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { makeTraitLabel } from 'e-teleport/AccessListManagement/Traits';
import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';
import { useFetchUserAndRoles } from 'e-teleport/AccessListManagement/useFetchUsersAndRoles';
import useTeleportE from 'e-teleport/useTeleportE';

import type { PropsWithChildren } from 'react';
import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import type TeleportEContext from 'e-teleport/teleportContextE';
import type { AccessList } from 'e-teleport/services/accessmanagement';

// PreProcessFn is a function that takes a list of AccessList and returns a list of AccessLists.
// This is used to modify the access lists before they are processed, e.g. to filter out certain lists
// or add additional information after changes are made in the web ui.
type PreProcessFn = (lists: AccessList[]) => AccessList[];

interface AccessListManagementContext {
  accessLists: AccessListWithModifiedGrants[];
  attempt: ReturnType<typeof useAttempt>;
  usersAndRolesAttempt: ReturnType<typeof useAttempt>['attempt'];
  processAccessLists: (preProcess?: PreProcessFn) => void;
  fetchRoleOptions: ReturnType<typeof useFetchUserAndRoles>['fetchRoleOptions'];
  fetchUsersAndRoles: ReturnType<
    typeof useFetchUserAndRoles
  >['fetchUsersAndRoles'];
  userOptions: ReturnType<typeof useFetchUserAndRoles>['userOptions'];
}

const STUB_ATTEMPT = {
  attempt: { status: '' },
  setAttempt: () => {},
  run: () => Promise.resolve(true),
  handleError: () => {},
} satisfies ReturnType<typeof useAttempt>;

const AccessListManagementContext = createContext<AccessListManagementContext>({
  attempt: STUB_ATTEMPT,
  usersAndRolesAttempt: STUB_ATTEMPT.attempt,
  accessLists: [],
  userOptions: [],
  processAccessLists: () => {},
  fetchRoleOptions: () => Promise.resolve([]),
  fetchUsersAndRoles: () => Promise.resolve(),
});

export const AccessListManagementContextProvider = (
  props: PropsWithChildren<unknown>
) => {
  const ctx = useTeleportE();
  const [accessLists, setAccessLists] = useState<
    AccessListWithModifiedGrants[]
  >([]);
  const initialFetch = useRef<Promise<void>>(null);
  const pendingPreProcessRef = useRef<PreProcessFn[]>([]);

  const attempt = useAttempt('processing');
  const usersAndRolesAttempt = useAttempt('processing');

  useEffect(() => {
    if (initialFetch.current) {
      return;
    }

    initialFetch.current = fetchAccessListsWithAttempt({
      ctx,
      attempt,
      setAccessLists,
      pendingPreProcessRef,
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const { userOptions, fetchRoleOptions, fetchUsersAndRoles } =
    useFetchUserAndRoles(usersAndRolesAttempt);

  const processAccessLists = (
    preProcess?: (lists: AccessList[]) => AccessList[]
  ) => {
    if (attempt.attempt.status === 'processing') {
      if (typeof preProcess === 'function') {
        pendingPreProcessRef.current.push(preProcess);
      }
      return;
    }

    processFetchedLists({ ctx, listsToUse: accessLists, setAccessLists })(
      preProcess
    );
  };

  return (
    <AccessListManagementContext.Provider
      value={{
        attempt,
        usersAndRolesAttempt: usersAndRolesAttempt.attempt,
        accessLists,
        userOptions,
        processAccessLists,
        fetchRoleOptions,
        fetchUsersAndRoles,
      }}
    >
      {props.children}
    </AccessListManagementContext.Provider>
  );
};

export const useAccessListManagementContext = () =>
  useContext(AccessListManagementContext);

const fetchAccessListsWithAttempt = async ({
  ctx,
  attempt,
  setAccessLists,
  pendingPreProcessRef,
}: {
  ctx: TeleportEContext;
  attempt: ReturnType<typeof useAttempt>;
  setAccessLists: (lists: AccessListWithModifiedGrants[]) => void;
  pendingPreProcessRef: { current: PreProcessFn[] };
}): Promise<void> => {
  attempt.setAttempt({ status: 'processing' });

  try {
    let listsToUse = await accessManagementService.fetchAccessLists();

    // If there are any pending pre-process functions, apply them to the lists in order.
    if (pendingPreProcessRef.current?.length) {
      pendingPreProcessRef.current.forEach(fn => (listsToUse = fn(listsToUse)));
      pendingPreProcessRef.current = [];
    }

    processFetchedLists({ ctx, listsToUse, setAccessLists })();

    attempt.setAttempt({ status: 'success' });
  } catch (e) {
    if (e.name === 'AbortError') {
      attempt.setAttempt({ status: 'success' });
      return;
    }
    if (e instanceof ApiError) {
      if (e.response.status === 403) {
        attempt.setAttempt({ status: '' });
        return;
      }
    }
    attempt.setAttempt({ status: 'failed', statusText: e.message });
  }
};

const processFetchedLists =
  ({
    ctx,
    listsToUse,
    setAccessLists,
  }: {
    ctx: TeleportEContext;
    listsToUse: AccessList[];
    setAccessLists: (lists: AccessListWithModifiedGrants[]) => void;
  }) =>
  (preProcess?: (lists: AccessList[]) => AccessList[]) => {
    if (typeof preProcess === 'function') {
      listsToUse = preProcess(listsToUse);
    }

    // Update notifications for access lists.
    ctx.storeNotifications.setNotificationsForAccessListsRequiringReview(
      listsToUse,
      ctx.storeUser.state
    );

    const processedLists = orderByNameAndReviewState(processTraits(listsToUse));

    setAccessLists(processedLists);
  };

const orderByNameAndReviewState = (lists: AccessListWithModifiedGrants[]) => {
  const sorted = lists.sort((a, b) =>
    compareByString(a.title.toLocaleLowerCase(), b.title.toLocaleLowerCase())
  );

  // Sort by required reviews by date.
  const noReviewsRequired = sorted.filter(l => !l.needsReviewBy);
  const requiresReviewSortedByDate = sorted
    .filter(l => l.needsReviewBy)
    .sort((a, b) => a.audit.nextDate.getTime() - b.audit.nextDate.getTime());

  return [...requiresReviewSortedByDate, ...noReviewsRequired];
};

const processTraits = (
  fetchedLists: (AccessList | AccessListWithModifiedGrants)[]
) => {
  const todayDate = new Date();
  const updatedAccessLists = fetchedLists.map(r => {
    const memberTraitList = [];
    const memberTraitKeys = Object.keys(r.grants.traits);
    if (memberTraitKeys.length > 0) {
      memberTraitKeys.forEach(key => {
        memberTraitList.push(makeTraitLabel(key, r.grants.traits[key]));
      });
    }
    const ownerTraitList = [];
    const ownerTraitKeys = Object.keys(r.ownerGrants.traits);
    if (ownerTraitKeys.length > 0) {
      ownerTraitKeys.forEach(key => {
        ownerTraitList.push(makeTraitLabel(key, r.ownerGrants.traits[key]));
      });
    }

    return {
      ...r,
      grants: { ...r.grants, traitList: memberTraitList.sort() },
      ownerGrants: { ...r.ownerGrants, traitList: ownerTraitList.sort() },
      needsReviewBy: accessListRequiresReview({
        todayDate,
        reviewDate: r.audit.nextDate,
      })
        ? r.audit.nextDate
        : null,
    };
  });

  return updatedAccessLists satisfies AccessListWithModifiedGrants[];
};
