import {
  keepPreviousData,
  useInfiniteQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { subWeeks } from 'date-fns';
import {
  createContext,
  Dispatch,
  SetStateAction,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type PropsWithChildren,
} from 'react';
import { useLocation, useNavigate } from 'react-router';

import { parseSortType } from 'design/DataTable/sort';
import type { SortDir } from 'design/DataTable/types';
import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import { makeTraitLabel } from 'e-teleport/AccessListManagement/Traits';
import {
  accessManagementService,
  type AccessList,
} from 'e-teleport/services/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import { ApiError } from 'teleport/services/api/parseError';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import { KeysEnum } from 'teleport/services/storageService';

import { GuideEditorState, useGuideEditor } from './GuideEditor/useGuideEditor';

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
  fieldName: 'title' | 'auditNextDate' | string;
  dir: SortDir;
};

type AccessListSearchParams = {
  search?: string;
  owners?: string[];
  roles?: string[];
  sort?: AccessListSort;
};

const listAccessListsQueryKey = 'listAccessListsV2';

type AccessListMutation =
  | { mutationType: 'created'; accessList: AccessList }
  | { mutationType: 'edited'; accessList: AccessList }
  | { mutationType: 'reviewed'; accessList: AccessList }
  | { mutationType: 'deleted'; accessListId: string };

export interface AccessListManagementContextValue {
  isOktaPluginReadOnly: boolean;
  search: string;
  processAccessLists: (
    fetchedLists: AccessList[],
    preProcess?: PreProcessFn
  ) => void;
  // backendCacheUnhealthy is true if the backend cache is unable to sort by title, or is disabled.
  // This is set if we receive at 412 status code error when fetching access lists.
  backendCacheUnhealthy: boolean;
  accessLists: AccessListWithModifiedGrants[];
  // if we are fetching a page.
  isFetching: boolean;
  // was there an error fetching accessLists
  isError: boolean;
  // the error fetching accessLists if exists
  error: Error | null;
  // refetch the current query
  refetch: () => void;
  updateSearchParams: (searchParams: AccessListSearchParams) => void;
  sort: AccessListSort;
  view: ViewMode;
  setView: Dispatch<SetStateAction<ViewMode>>;
  filters: AccessListFilters;
  filtersExist?: boolean;
  isFetchingNextPage?: boolean;
  fetchNextPage?: () => void;
  hasNextPage?: boolean;
  updateAccessListCache: (mutation: AccessListMutation) => void;
  previousSearchParams?: string;
  guideEditor: GuideEditorState;
  /**
   * Used to determine if okta plugin has been created.
   * Error or no result (null) will be interpreted as "not created".
   *
   * Intended to determine if CTA is needed to lead user to create
   * a okta plugin since okta apps/groups can be synced as access lists as well.
   *
   * Also can be used to get more information for Okta originated access lists.
   */
  oktaPluginAttempt: Attempt<Plugin<PluginOktaSpec, PluginStatusOkta>>;
}

const DEFAULT_SORT = {
  fieldName: 'title',
  dir: 'ASC',
} satisfies AccessListSort;

export const AccessListManagementContext =
  createContext<AccessListManagementContextValue>({
    refetch: () => {},
    isOktaPluginReadOnly: false,
    accessLists: [],
    backendCacheUnhealthy: false,
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    search: '',
    filters: {},
    sort: DEFAULT_SORT,
    view: ViewMode.CARD,
    error: null,
    isError: false,
    setView: () => {},
    processAccessLists: () => {},
    updateSearchParams: () => {},
    previousSearchParams: '',
    filtersExist: false,
    updateAccessListCache: () => {},
    guideEditor: {
      reset: () => {},
      preset: '',
      setPreset: () => {},
      currentStep: 0,
      setCurrentStep: () => {},
      prevStep: () => {},
      nextStep: () => {},
      awsIcRoleState: undefined,
      standardRoleState: undefined,
      definedAccess: () => false,
      definedAccessInAnyRoleCondition: () => false,
      undoEditRoleChanges: () => {},
      isEditing: false,
      getRolesToSave: () => [],
      getResumableState: () => null,
      removeLocationState: () => null,
      originatedFromOkta: false,
    },
    oktaPluginAttempt: undefined,
  });

export const AccessListManagementContextProvider = (
  props: PropsWithChildren<unknown>
) => {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const guideEditor = useGuideEditor();

  const [backendCacheUnhealthy, setBackendCacheUnhealthy] = useState(false);
  const [previousSearchParams, setPreviousSearchParams] = useState('');
  const accessListPreferences =
    JSON.parse(
      localStorage.getItem(KeysEnum.ACCESS_LIST_PREFERENCES) || '{}'
    ) || {};

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

  const pendingPreProcessRef = useRef<PreProcessFn[]>([]);

  const queryParams = new URLSearchParams(location.search);
  const search = queryParams.get('search');

  const sortParam = queryParams.get('sort') || 'title:asc';
  const sort = parseSortType(sortParam);

  const owners = queryParams.getAll('owners');
  const filtersExist = owners.length > 0 || !!search;

  const pageSize = 48;
  const {
    data,
    fetchNextPage,
    hasNextPage,
    refetch,
    isFetching,
    isFetchingNextPage,
    error,
    isError,
  } = useInfiniteQuery({
    queryKey: [listAccessListsQueryKey, sort, search, owners],
    queryFn: async ({ pageParam, signal }) => {
      const currentParams = new URLSearchParams(location.search);
      const currentSearch = currentParams.get('search');
      const currentSortParam = currentParams.get('sort') || 'title:asc';
      const currentSort = backendCacheUnhealthy
        ? parseSortType('name:asc')
        : parseSortType(currentSortParam);
      const currentOwners = currentParams.getAll('owners');
      return accessManagementService.fetchAccessListsV2(
        {
          limit: pageSize,
          startKey: pageParam,
          sort: currentSort,
          search: currentSearch,
          owners: currentOwners,
        },
        signal
      );
    },
    initialPageParam: '',
    getNextPageParam: data => data?.startKey || undefined,
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  useEffect(() => {
    // cache is unhealthy or disabled, we want to inform the user
    // of the degradation of the service. If they sent an arbitrary sort
    // param like "blahblah" tho, we can still just show the normal error.
    if (
      error instanceof ApiError &&
      error.response.status === 412 &&
      (sort.fieldName === 'title' || sort.fieldName === 'auditNextDate')
    ) {
      setBackendCacheUnhealthy(true);
      refetch();
    }
  }, [error, refetch, sort]);

  const updateSearchParams = useCallback(
    ({ search, owners, sort }: AccessListSearchParams) => {
      const params = new URLSearchParams(location.search);

      if (search !== undefined) {
        if (search && search.trim() !== '') {
          params.set('search', search);
        } else {
          params.delete('search');
        }
      }

      if (owners !== undefined) {
        params.delete('owners');
        if (owners.length > 0) {
          owners.forEach(owner => params.append('owners', owner));
        }
      }

      if (sort !== undefined) {
        if (sort && sort.fieldName && sort.dir) {
          params.set('sort', `${sort.fieldName}:${sort.dir}`);
        } else {
          params.delete('sort');
        }
      }

      setPreviousSearchParams(params.toString());
      navigate(
        {
          pathname: location.pathname,
          search: params.toString(),
        },
        { replace: true }
      );
    },
    [navigate, location.search, location.pathname]
  );

  const [oktaPluginAttempt, fetchOktaPlugin] = useAsync<
    [],
    Plugin<PluginOktaSpec, PluginStatusOkta>
  >(
    useCallback(
      () =>
        pluginsService.fetchPlugin('okta').catch(err => {
          if (err instanceof ApiError && err.response.status === 404) {
            return undefined;
          }
          throw err;
        }),
      []
    )
  );

  useEffect(() => {
    if (['success', 'processing'].includes(oktaPluginAttempt.status)) {
      return;
    }

    void fetchOktaPlugin();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const processAccessLists = (
    fetchedLists: AccessList[],
    preProcess?: (lists: AccessList[]) => AccessList[]
  ) => {
    if (isFetching) {
      if (typeof preProcess === 'function') {
        pendingPreProcessRef.current.push(preProcess);
      }
      return {
        accessLists: processFetchedLists({ listsToUse: fetchedLists })(
          preProcess
        ),
      };
    }

    return {
      accessLists: processFetchedLists({ listsToUse: fetchedLists })(
        preProcess
      ),
    };
  };

  const { accessLists } = processAccessLists(
    data?.pages.flatMap(page => page.agents) || []
  );

  const updateAccessListCache = useCallback(
    (mutation: AccessListMutation) => {
      const queryKey = [listAccessListsQueryKey];

      queryClient.setQueriesData({ queryKey }, (oldData: any) => {
        if (!oldData) return oldData;

        let newPages = [...oldData.pages];
        switch (mutation.mutationType) {
          case 'created':
            if (newPages.length > 0) {
              newPages[0] = {
                ...newPages[0],
                agents: [mutation.accessList, ...newPages[0].agents],
              };
            }
            break;
          case 'reviewed': // same as edited, but leaving for logic elsewhere if needed
          case 'edited':
            newPages = newPages.map((page: any) => ({
              ...page,
              agents: page.agents.map((list: AccessList) =>
                list.id === mutation.accessList.id ? mutation.accessList : list
              ),
            }));
            break;
          case 'deleted':
            newPages = newPages.map((page: any) => ({
              ...page,
              agents: page.agents.filter(
                (list: AccessList) => list.id !== mutation.accessListId
              ),
            }));
            break;
        }
        return { ...oldData, pages: newPages };
      });
    },
    [queryClient]
  );

  return (
    <AccessListManagementContext.Provider
      value={{
        isFetching,
        backendCacheUnhealthy,
        isError,
        refetch: () => refetch(),
        isFetchingNextPage,
        hasNextPage,
        fetchNextPage,
        error,
        accessLists,
        // Okta Integration is read-only if bidirectionalSync is 'false' or omitted.
        isOktaPluginReadOnly:
          oktaPluginAttempt?.data &&
          !oktaPluginAttempt.data.spec?.enableBidirectionalSync,
        oktaPluginAttempt,
        updateSearchParams,
        search,
        filters: {
          owners,
        },
        view,
        setView,
        filtersExist,
        processAccessLists,
        sort,
        updateAccessListCache,
        previousSearchParams,
        guideEditor,
      }}
    >
      {props.children}
    </AccessListManagementContext.Provider>
  );
};

export const useAccessListManagementContext = () =>
  useContext(AccessListManagementContext);

const processFetchedLists =
  ({ listsToUse }: { listsToUse: AccessList[] }) =>
  (preProcess?: (lists: AccessList[]) => AccessList[]) => {
    if (typeof preProcess === 'function') {
      listsToUse = preProcess(listsToUse);
    }

    return processTraits(listsToUse);
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

export function accessListRequiresReview({
  todayDate,
  reviewDate,
}: {
  todayDate: Date;
  reviewDate: Date | undefined;
}) {
  if (!reviewDate) {
    return false;
  }

  return todayDate >= subWeeks(reviewDate, 2);
}
