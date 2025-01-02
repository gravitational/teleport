import {
  createContext,
  Dispatch,
  SetStateAction,
  useContext,
  useEffect,
  useRef,
  useState,
  type PropsWithChildren,
} from 'react';

import type { SortDir } from 'design/DataTable/types';
import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import useAttempt from 'shared/hooks/useAttemptNext';

import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import { makeTraitLabel } from 'e-teleport/AccessListManagement/Traits';
import { useFetchUserAndRoles } from 'e-teleport/AccessListManagement/useFetchUsersAndRoles';
import {
  AccessListOwner,
  accessManagementService,
  type AccessList,
} from 'e-teleport/services/accessmanagement';
import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';
import type TeleportEContext from 'e-teleport/teleportContextE';
import useTeleportE from 'e-teleport/useTeleportE';
import { ApiError } from 'teleport/services/api/parseError';
import { KeysEnum } from 'teleport/services/storageService';

// PreProcessFn is a function that takes a list of AccessList and returns a list of AccessLists.
// This is used to modify the access lists before they are processed, e.g. to filter out certain lists
// or add additional information after changes are made in the web ui.
type PreProcessFn = (lists: AccessList[]) => AccessList[];

// AccessListFilters is used to filter the access lists based on the source, owners, and roles.
export type AccessListFilters = {
  source?: ('okta' | 'teleport' | 'aws-identity-center')[];
  owners?: string[];
  roles?: string[];
};

// AccessListSort is used to sort the access lists based on a property name and direction.
export type AccessListSort = {
  fieldName: keyof AccessListWithModifiedGrants;
  dir: SortDir;
};

type State = {
  accessLists: AccessListWithModifiedGrants[];
  allOwners: AccessListOwner[];
  allGrantedRoles: string[];
};

interface AccessListManagementContext {
  attempt: ReturnType<typeof useAttempt>;
  usersAndRolesAttempt: ReturnType<typeof useAttempt>['attempt'];
  processAccessLists: (preProcess?: PreProcessFn) => void;
  fetchRoleOptions: ReturnType<typeof useFetchUserAndRoles>['fetchRoleOptions'];
  fetchUsersAndRoles: ReturnType<
    typeof useFetchUserAndRoles
  >['fetchUsersAndRoles'];
  refetchAccessLists: (setAttempt: boolean) => void;

  accessLists: State['accessLists'];
  allOwners: State['allOwners'];
  allGrantedRoles: State['allGrantedRoles'];
  userOptions: ReturnType<typeof useFetchUserAndRoles>['userOptions'];
  filters: AccessListFilters;
  sort: AccessListSort;
  view: ViewMode;
  setFilters: Dispatch<SetStateAction<AccessListFilters>>;
  setSort: Dispatch<SetStateAction<AccessListSort>>;
  setView: Dispatch<SetStateAction<ViewMode>>;
}

const STUB_ATTEMPT = {
  attempt: { status: '' },
  setAttempt: () => {},
  run: () => Promise.resolve(true),
  handleError: () => {},
} satisfies ReturnType<typeof useAttempt>;

const DEFAULT_SORT = {
  fieldName: 'title',
  dir: 'ASC',
} satisfies AccessListSort;

const AccessListManagementContext = createContext<AccessListManagementContext>({
  attempt: STUB_ATTEMPT,
  usersAndRolesAttempt: STUB_ATTEMPT.attempt,
  accessLists: [],
  userOptions: [],
  allOwners: [],
  allGrantedRoles: [],
  filters: {},
  sort: DEFAULT_SORT,
  view: ViewMode.CARD,
  setView: () => {},
  setFilters: () => {},
  setSort: () => {},
  processAccessLists: () => {},
  fetchRoleOptions: () => Promise.resolve([]),
  fetchUsersAndRoles: () => Promise.resolve(),
  refetchAccessLists: () => {},
});

export const AccessListManagementContextProvider = (
  props: PropsWithChildren<unknown>
) => {
  const ctx = useTeleportE();

  const accessListPreferences =
    JSON.parse(
      localStorage.getItem(KeysEnum.ACCESS_LIST_PREFERENCES) || '{}'
    ) || {};

  const [state, setState] = useState<State>({
    accessLists: [],
    allOwners: [],
    allGrantedRoles: [],
  });

  const [filters, setFilters] = useState<AccessListFilters>(
    accessListPreferences?.filters || {}
  );
  const [sort, setSort] = useState<AccessListSort>(
    accessListPreferences?.sort || DEFAULT_SORT
  );
  const [view, _setView] = useState<ViewMode>(
    accessListPreferences?.view || ViewMode.CARD
  );
  const setView = (newState: ViewMode) => {
    _setView(newState);

    if (accessListPreferences?.view !== newState) {
      localStorage.setItem(
        KeysEnum.ACCESS_LIST_PREFERENCES,
        JSON.stringify({
          ...accessListPreferences,
          view: newState,
        })
      );
    }
  };

  const initialFetch = useRef<Promise<void>>(null);
  const pendingPreProcessRef = useRef<PreProcessFn[]>([]);

  const attempt = useAttempt('processing');
  const usersAndRolesAttempt = useAttempt('processing');

  const refetchAccessLists = (setAttempt: boolean) => {
    initialFetch.current = fetchAccessListsWithAttempt({
      ctx,
      attempt,
      pendingPreProcessRef,
      setAttempt,
      setState,
    });
  };

  useEffect(() => {
    if (initialFetch.current) {
      return;
    }

    refetchAccessLists(true);
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

    setState(prev => {
      const accessLists = processFetchedLists({
        ctx,
        listsToUse: prev.accessLists,
      })(preProcess);
      const { allOwners, allGrantedRoles } =
        getOwnersRolesFromLists(accessLists);
      return {
        accessLists,
        allOwners,
        allGrantedRoles,
      };
    });
  };

  return (
    <AccessListManagementContext.Provider
      value={{
        attempt,
        usersAndRolesAttempt: usersAndRolesAttempt.attempt,
        userOptions,
        accessLists: state.accessLists,
        allOwners: state.allOwners,
        allGrantedRoles: state.allGrantedRoles,
        filters,
        sort,
        view,
        setFilters,
        setSort,
        setView,
        processAccessLists,
        fetchRoleOptions,
        fetchUsersAndRoles,
        refetchAccessLists,
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
  pendingPreProcessRef,
  setState,
  setAttempt = true,
}: {
  ctx: TeleportEContext;
  attempt: ReturnType<typeof useAttempt>;
  pendingPreProcessRef: { current: PreProcessFn[] };
  setState: Dispatch<SetStateAction<State>>;
  setAttempt?: boolean;
}): Promise<void> => {
  if (setAttempt) {
    attempt.setAttempt({ status: 'processing' });
  }

  try {
    let listsToUse = await accessManagementService.fetchAccessLists();

    // If there are any pending pre-process functions, apply them to the lists in order.
    if (pendingPreProcessRef.current?.length) {
      pendingPreProcessRef.current.forEach(fn => (listsToUse = fn(listsToUse)));
      pendingPreProcessRef.current = [];
    }

    const processedLists = processFetchedLists({ ctx, listsToUse })();
    const { allOwners, allGrantedRoles } =
      getOwnersRolesFromLists(processedLists);

    setState({
      accessLists: processedLists,
      allOwners,
      allGrantedRoles,
    });
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
  ({ ctx, listsToUse }: { ctx: TeleportEContext; listsToUse: AccessList[] }) =>
  (preProcess?: (lists: AccessList[]) => AccessList[]) => {
    if (typeof preProcess === 'function') {
      listsToUse = preProcess(listsToUse);
    }

    // Update notifications for access lists.
    ctx.storeNotifications.setNotificationsForAccessListsRequiringReview(
      listsToUse,
      ctx.storeUser.state
    );

    return processTraits(listsToUse);
  };

const getOwnersRolesFromLists = (lists: AccessList[]) => {
  const allOwners: AccessListOwner[] = [];
  const allGrantedRoles: string[] = [];

  for (let i = 0; i < lists.length; i++) {
    for (const owner of lists[i].owners) {
      if (!allOwners.some(o => o.name === owner.name)) {
        allOwners.push(owner);
      }
    }
    for (const role of lists[i].grants.roles) {
      if (!allGrantedRoles.includes(role)) {
        allGrantedRoles.push(role);
      }
    }
  }

  return { allOwners, allGrantedRoles };
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
      auditNextDate: r.audit.nextDate,
    };
  });

  return updatedAccessLists satisfies AccessListWithModifiedGrants[];
};
