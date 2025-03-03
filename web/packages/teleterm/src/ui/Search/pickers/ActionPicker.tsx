/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { ReactElement, useCallback, useMemo } from 'react';
import styled from 'styled-components';

import { Box, ButtonBorder, Label as DesignLabel, Flex, Text } from 'design';
import * as icons from 'design/Icon';
import { Cross as CloseIcon } from 'design/Icon';
import { AdvancedSearchToggle } from 'shared/components/AdvancedSearchToggle';
import { Highlight } from 'shared/components/Highlight';
import {
  Attempt,
  hasFinished,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import { isWebApp } from 'teleterm/services/tshd/app';
import * as tsh from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  DisplayResults,
  isClusterSearchFilter,
  ResourceMatch,
  ResourceSearchResult,
  SearchFilter,
  SearchResult,
  SearchResultApp,
  SearchResultCluster,
  SearchResultDatabase,
  SearchResultKube,
  SearchResultResourceType,
  SearchResultServer,
} from 'teleterm/ui/Search/searchResult';
import { ResourceSearchError } from 'teleterm/ui/services/resources';
import * as uri from 'teleterm/ui/uri';
import { assertUnreachable } from 'teleterm/ui/utils';
import { isRetryable } from 'teleterm/ui/utils/retryWithRelogin';
import { useVnetContext } from 'teleterm/ui/Vnet';

import { SearchAction } from '../actions';
import { useSearchContext } from '../SearchContext';
import {
  CrossClusterResourceSearchResult,
  resourceTypeToReadableName,
} from '../useSearch';
import { PickerContainer } from './PickerContainer';
import { getParameterPicker } from './pickers';
import { IconAndContent, NonInteractiveItem, ResultList } from './ResultList';
import { useActionAttempts } from './useActionAttempts';

export function ActionPicker(props: { input: ReactElement }) {
  const ctx = useAppContext();
  const { clustersService, modalsService } = ctx;
  ctx.clustersService.useState();

  const {
    changeActivePicker,
    pauseUserInteraction,
    close,
    inputValue,
    resetInput,
    filters,
    removeFilter,
    addWindowEventListener,
    advancedSearchEnabled,
    toggleAdvancedSearch,
  } = useSearchContext();
  const {
    displayResultsAction,
    filterActions,
    resourceActionsAttempt,
    resourceSearchAttempt,
  } = useActionAttempts();
  const { isSupported: isVnetSupported } = useVnetContext();
  const totalCountOfClusters = clustersService.getClusters().length;

  const getClusterName = useCallback(
    (resourceUri: uri.ClusterOrResourceUri) => {
      const clusterUri = uri.routing.ensureClusterUri(resourceUri);
      const cluster = clustersService.findCluster(clusterUri);

      return cluster ? cluster.name : uri.routing.parseClusterName(resourceUri);
    },
    [clustersService]
  );

  const getOptionalClusterName = useCallback(
    (resourceUri: uri.ClusterOrResourceUri) =>
      totalCountOfClusters === 1 ? undefined : getClusterName(resourceUri),
    [getClusterName, totalCountOfClusters]
  );

  const onPick = useCallback(
    (action: SearchAction) => {
      if (action.type === 'simple-action') {
        action.perform();
        // TODO: This logic probably should be encapsulated inside SearchContext, so that ActionPicker
        // and ParameterPicker can reuse it.
        //
        // Overall, the context should probably encapsulate more logic so that the components don't
        // have to worry about low-level stuff such as input state. Input state already lives in the
        // search context so it should be managed from there, if possible.
        if (!action.preventAutoInputReset) {
          resetInput();
        }
        if (!action.preventAutoClose) {
          close();
        }
      }
      if (action.type === 'parametrized-action') {
        changeActivePicker(getParameterPicker(action));
      }
    },
    [changeActivePicker, close, resetInput]
  );

  const filterButtons = filters.map(s => {
    if (s.filter === 'resource-type') {
      return (
        <FilterButton
          key={`resource-type-${s.resourceType}`}
          text={resourceTypeToReadableName[s.resourceType]}
          onClick={() => removeFilter(s)}
        />
      );
    }
    if (s.filter === 'cluster') {
      const clusterName = getClusterName(s.clusterUri);
      return (
        <FilterButton
          key="cluster"
          text={clusterName}
          onClick={() => removeFilter(s)}
        />
      );
    }
  });

  function handleKeyDown(e: React.KeyboardEvent) {
    const { length } = filters;
    if (e.key === 'Backspace' && inputValue === '' && length) {
      removeFilter(filters[length - 1]);
    }
  }

  const actionPickerStatus = useMemo(
    () =>
      getActionPickerStatus({
        inputValue,
        filters,
        filterActions,
        resourceSearchAttempt,
        allClusters: clustersService.getClusters(),
      }),
    [inputValue, filters, filterActions, resourceSearchAttempt, clustersService]
  );
  const showErrorsInModal = useCallback(
    errors =>
      pauseUserInteraction(
        () =>
          new Promise(resolve => {
            modalsService.openRegularDialog({
              kind: 'resource-search-errors',
              errors,
              getClusterName,
              onCancel: () => resolve(undefined),
            });
          })
      ),
    [pauseUserInteraction, modalsService, getClusterName]
  );

  // The order of attempts is important.
  // Display results action and filter actions should be displayed before resource actions.
  const resultListAttempts = [
    makeSuccessAttempt([displayResultsAction]),
    makeSuccessAttempt(filterActions),
    resourceActionsAttempt,
  ];

  return (
    <PickerContainer>
      <InputWrapper onKeyDown={handleKeyDown}>
        {filterButtons}
        {props.input}
      </InputWrapper>
      <ResultList<SearchAction>
        attempts={resultListAttempts}
        onPick={onPick}
        onBack={close}
        addWindowEventListener={addWindowEventListener}
        render={item => {
          const Component = ComponentMap[item.searchResult.kind];
          return {
            key: getKey(item.searchResult),
            Component: (
              <Component
                searchResult={item.searchResult}
                getOptionalClusterName={getOptionalClusterName}
                isVnetSupported={isVnetSupported}
              />
            ),
          };
        }}
        ExtraTopComponent={
          <ExtraTopComponents
            status={actionPickerStatus}
            getClusterName={getClusterName}
            showErrorsInModal={showErrorsInModal}
            advancedSearch={{
              isToggled: advancedSearchEnabled,
              onToggle: toggleAdvancedSearch,
            }}
          />
        }
      />
    </PickerContainer>
  );
}

function getKey(searchResult: SearchResult): string {
  switch (searchResult.kind) {
    case 'resource-type-filter':
      return `${searchResult.kind}-${searchResult.resource}`;
    case 'display-results':
      return `${searchResult.kind}-${searchResult.documentUri}-${searchResult.value}`;
    default:
      return `${searchResult.kind}-${searchResult.resource.uri}`;
  }
}

export const InputWrapper = styled(Flex).attrs({ px: 2 })`
  row-gap: ${props => props.theme.space[2]}px;
  column-gap: ${props => props.theme.space[2]}px;
  align-items: center;
  flex-wrap: wrap;
  // account for border
  padding-block: calc(${props => props.theme.space[2]}px - 1px);
  // input height without border
  min-height: 38px;

  & > input {
    height: unset;
    padding-inline: 0;
    flex: 1;
  }
`;

const ExtraTopComponents = (props: {
  status: ActionPickerStatus;
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  showErrorsInModal: (errors: ResourceSearchError[]) => void;
  advancedSearch: AdvancedSearch;
}) => {
  const { status, getClusterName, showErrorsInModal, advancedSearch } = props;

  if (advancedSearch.isToggled) {
    return <AdvancedSearchEnabledItem advancedSearch={advancedSearch} />;
  }

  switch (status.inputState) {
    case 'no-input': {
      switch (status.searchMode.kind) {
        case 'no-search': {
          return (
            <TypeToSearchItem
              hasNoRemainingFilterActions={false}
              advancedSearch={advancedSearch}
            />
          );
        }
        case 'preview': {
          const {
            nonRetryableResourceSearchErrors,
            hasNoRemainingFilterActions,
          } = status.searchMode;

          return (
            <>
              <TypeToSearchItem
                hasNoRemainingFilterActions={hasNoRemainingFilterActions}
                advancedSearch={advancedSearch}
              />
              {nonRetryableResourceSearchErrors.length > 0 && (
                <ResourceSearchErrorsItem
                  errors={nonRetryableResourceSearchErrors}
                  getClusterName={getClusterName}
                  showErrorsInModal={() => {
                    showErrorsInModal(nonRetryableResourceSearchErrors);
                  }}
                  // We show the advanced search in TypeToSearchItem.
                  advancedSearch={undefined}
                />
              )}
            </>
          );
        }
        default: {
          return assertUnreachable(status.searchMode);
        }
      }
    }
    case 'some-input': {
      const shouldShowResourceSearchErrorsItem =
        status.nonRetryableResourceSearchErrors.length > 0;
      const shouldShowNoResultsItem = status.hasNoResults;
      const shouldShowTypeToSearchItem =
        !shouldShowResourceSearchErrorsItem && !shouldShowNoResultsItem;

      return (
        <>
          {shouldShowResourceSearchErrorsItem && (
            <ResourceSearchErrorsItem
              errors={status.nonRetryableResourceSearchErrors}
              getClusterName={getClusterName}
              showErrorsInModal={() => {
                showErrorsInModal(status.nonRetryableResourceSearchErrors);
              }}
              advancedSearch={advancedSearch}
            />
          )}
          {shouldShowNoResultsItem && (
            <NoResultsItem
              clustersWithExpiredCerts={status.clustersWithExpiredCerts}
              getClusterName={getClusterName}
              // Show the toggle only
              // when ResourceSearchErrorsItem is not visible
              advancedSearch={
                shouldShowResourceSearchErrorsItem ? undefined : advancedSearch
              }
            />
          )}
          {shouldShowTypeToSearchItem && (
            <TypeToSearchItem
              hasNoRemainingFilterActions={false}
              advancedSearch={advancedSearch}
            />
          )}
        </>
      );
    }
    default: {
      assertUnreachable(status);
    }
  }
};

/**
 * ActionPickerStatus helps with displaying ExtraTopComponents. It has two goals:
 *
 *   * Encapsulate business logic so that anything that ExtraTopComponents renders can just read
 *     ActionPickerStatus fields.
 *   * Represent only valid UI states. For example, inputState 'no-input' doesn't have hasNoResults
 *     field as this field would make no sense in a situation where no search requests were made.
 *
 * As you may notice, ActionPickerStatus doesn't say whether the search request is in progress or
 * not, simply because displaying the progress bar is handled by another component. The questions
 * answered by ActionPickerStatus are valid to ask no matter what the state of the request is.
 */
type ActionPickerStatus =
  | {
      // no-input: The input is empty.
      inputState: 'no-input';
      searchMode:
        | {
            // no-search: The search bar is pristine, that is the input and the filters are empty.
            kind: 'no-search';
          }
        | {
            // preview: At least one filter is selected. The search bar is fetching or shows
            // a preview of results matching the filters.
            kind: 'preview';
            hasNoRemainingFilterActions: boolean;
            nonRetryableResourceSearchErrors: ResourceSearchError[];
          };
    }
  | {
      // some-input: The input is not empty. The search bar is fetching or shows results matching
      // the query and filters.
      inputState: 'some-input';
      hasNoResults: boolean;
      nonRetryableResourceSearchErrors: ResourceSearchError[];
      clustersWithExpiredCerts: Set<uri.ClusterUri>;
    };

export function getActionPickerStatus({
  inputValue,
  filters,
  filterActions,
  allClusters,
  resourceSearchAttempt,
}: {
  inputValue: string;
  filters: SearchFilter[];
  filterActions: SearchAction[];
  allClusters: tsh.Cluster[];
  resourceSearchAttempt: Attempt<CrossClusterResourceSearchResult>;
}): ActionPickerStatus {
  if (!inputValue) {
    const didNotSelectAnyFilters = filters.length === 0;

    // If the input is empty, we fetch the preview only after the user selected some filters.
    // So at this point we know that no search request was sent.
    if (didNotSelectAnyFilters) {
      return {
        inputState: 'no-input',
        searchMode: { kind: 'no-search' },
      };
    }

    // The number of available filters the user can select changes dynamically based on how many
    // clusters are in the state. That's why instead of inspecting the filters array from
    // SearchContext, we inspect the actual filter actions attempt to see if any further filter
    // suggestions will be shown to the user.
    //
    // We also know that this attempt is always successful as filters are calculated in a sync way.
    // They're converted into an attempt only to conform to the interface of ResultList.
    const hasNoRemainingFilterActions = filterActions.length === 0;

    const nonRetryableResourceSearchErrors =
      resourceSearchAttempt.status === 'success'
        ? resourceSearchAttempt.data.errors.filter(
            err => !isRetryable(err.cause)
          )
        : [];

    return {
      inputState: 'no-input',
      searchMode: {
        kind: 'preview',
        hasNoRemainingFilterActions,
        nonRetryableResourceSearchErrors,
      },
    };
  }

  const nonRetryableResourceSearchErrors = [];
  let clustersWithExpiredCerts = new Set(
    allClusters.filter(c => !c.connected).map(c => c.uri)
  );

  if (!hasFinished(resourceSearchAttempt)) {
    return {
      inputState: 'some-input',
      hasNoResults: false,
      nonRetryableResourceSearchErrors,
      clustersWithExpiredCerts,
    };
  }

  // resourceSearchAttempt never has error status.
  const hasNoResults =
    resourceSearchAttempt.data.results.length === 0 &&
    filterActions.length === 0;

  resourceSearchAttempt.data.errors.forEach(err => {
    if (isRetryable(err.cause)) {
      clustersWithExpiredCerts.add(err.clusterUri);
    } else {
      nonRetryableResourceSearchErrors.push(err);
    }
  });

  // Make sure we don't list extra clusters with expired certs if a cluster filter is selected.
  const clusterFilter = filters.find(isClusterSearchFilter);
  if (clusterFilter) {
    const hasClusterCertExpired = clustersWithExpiredCerts.has(
      clusterFilter.clusterUri
    );
    clustersWithExpiredCerts = new Set();

    if (hasClusterCertExpired) {
      clustersWithExpiredCerts.add(clusterFilter.clusterUri);
    }
  }

  return {
    inputState: 'some-input',
    hasNoResults,
    clustersWithExpiredCerts,
    nonRetryableResourceSearchErrors,
  };
}

export const ComponentMap: Record<
  SearchResult['kind'],
  React.FC<SearchResultItem<SearchResult>>
> = {
  server: ServerItem,
  kube: KubeItem,
  database: DatabaseItem,
  app: AppItem,
  'cluster-filter': ClusterFilterItem,
  'resource-type-filter': ResourceTypeFilterItem,
  'display-results': DisplayResultsItem,
};

type SearchResultItem<T> = {
  searchResult: T;
  getOptionalClusterName: (uri: uri.ClusterOrResourceUri) => string;
  isVnetSupported: boolean;
};

function ClusterFilterItem(props: SearchResultItem<SearchResultCluster>) {
  return (
    <IconAndContent Icon={icons.Lan} iconColor="text.slightlyMuted">
      <Text typography="body2">
        Search only in{' '}
        <strong>
          <Highlight
            text={props.searchResult.resource.name}
            keywords={[props.searchResult.nameMatch]}
          />
        </strong>
      </Text>
    </IconAndContent>
  );
}

function DisplayResultsItem(props: SearchResultItem<DisplayResults>) {
  return (
    <IconAndContent Icon={icons.Magnifier} iconColor="text.slightlyMuted">
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body2">
          Display {props.searchResult.value ? 'search' : 'all'} results{' '}
          {props.searchResult.value && (
            <>
              for{' '}
              <strong>
                <Highlight
                  keywords={[props.searchResult.value]}
                  text={props.searchResult.value}
                />
              </strong>
            </>
          )}
          {props.searchResult.documentUri
            ? ' in the current tab'
            : ' in a new tab'}
        </Text>
        <Box ml="auto">
          <Text typography="body4">
            {props.getOptionalClusterName(props.searchResult.clusterUri)}
          </Text>
        </Box>
      </Flex>
    </IconAndContent>
  );
}

const resourceIcons: Record<
  SearchResultResourceType['resource'],
  React.ComponentType<{
    color: string;
    fontSize: string;
    lineHeight: string;
  }>
> = {
  kube_cluster: icons.Kubernetes,
  node: icons.Server,
  db: icons.Database,
  app: icons.Application,
};

function ResourceTypeFilterItem(
  props: SearchResultItem<SearchResultResourceType>
) {
  return (
    <IconAndContent
      Icon={resourceIcons[props.searchResult.resource]}
      iconColor="text.slightlyMuted"
    >
      <Text typography="body2">
        Search for{' '}
        <strong>
          <Highlight
            text={resourceTypeToReadableName[props.searchResult.resource]}
            keywords={[props.searchResult.nameMatch]}
          />
        </strong>
      </Text>
    </IconAndContent>
  );
}

export function ServerItem(props: SearchResultItem<SearchResultServer>) {
  const { searchResult } = props;
  const server = searchResult.resource;
  const hasUuidMatches = searchResult.resourceMatches.some(
    match => match.field === 'name'
  );

  return (
    <IconAndContent
      Icon={icons.Server}
      iconColor="brand"
      iconOpacity={getRequestableResourceIconOpacity(props.searchResult)}
    >
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body2">
          {props.searchResult.requiresRequest
            ? 'Request access to server '
            : 'Connect over SSH to '}
          <strong>
            <HighlightField field="hostname" searchResult={searchResult} />
          </strong>
        </Text>
        <Box ml="auto">
          <Text typography="body4">
            {props.getOptionalClusterName(server.uri)}
          </Text>
        </Box>
      </Flex>

      <Labels searchResult={searchResult}>
        <ResourceFields>
          {server.tunnel ? (
            <span title="This node is connected to the cluster through a reverse tunnel">
              ↵ tunnel
            </span>
          ) : (
            <span>
              <HighlightField field="addr" searchResult={searchResult} />
            </span>
          )}

          {hasUuidMatches && (
            <span>
              UUID:{' '}
              <HighlightField field={'name'} searchResult={searchResult} />
            </span>
          )}
        </ResourceFields>
      </Labels>
    </IconAndContent>
  );
}

export function DatabaseItem(props: SearchResultItem<SearchResultDatabase>) {
  const { searchResult } = props;
  const db = searchResult.resource;

  const $resourceFields = (
    <ResourceFields>
      <span
        css={`
          flex-shrink: 0;
        `}
      >
        <HighlightField field="type" searchResult={searchResult} />
        /
        <HighlightField field="protocol" searchResult={searchResult} />
      </span>
      {db.desc && (
        <span
          css={`
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          <HighlightField field="desc" searchResult={searchResult} />
        </span>
      )}
    </ResourceFields>
  );

  return (
    <IconAndContent
      Icon={icons.Database}
      iconColor="brand"
      iconOpacity={getRequestableResourceIconOpacity(props.searchResult)}
    >
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body2">
          {props.searchResult.requiresRequest
            ? 'Request access to db '
            : 'Set up a db connection to '}
          <strong>
            <HighlightField field="name" searchResult={searchResult} />
          </strong>
        </Text>
        <Box ml="auto">
          <Text typography="body4">{props.getOptionalClusterName(db.uri)}</Text>
        </Box>
      </Flex>

      {/* If the description is long, put the resource fields on a separate line.
          Otherwise show the resource fields and the labels together in a single line.
       */}
      {db.desc.length >= 30 ? (
        <>
          {$resourceFields}
          <Labels searchResult={searchResult} />
        </>
      ) : (
        <Labels searchResult={searchResult}>{$resourceFields}</Labels>
      )}
    </IconAndContent>
  );
}

export function AppItem(props: SearchResultItem<SearchResultApp>) {
  const { searchResult } = props;
  const app = searchResult.resource;

  const $appName = (
    <strong>
      <HighlightField
        field={app.friendlyName ? 'friendlyName' : 'name'}
        searchResult={searchResult}
      />
    </strong>
  );

  const $resourceFields = (app.addrWithProtocol || app.desc) && (
    <ResourceFields>
      {app.addrWithProtocol && (
        <span
          css={`
            flex-shrink: 0;
          `}
        >
          <HighlightField
            field="addrWithProtocol"
            searchResult={searchResult}
          />
        </span>
      )}
      {app.desc && (
        <span
          css={`
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          <HighlightField field="desc" searchResult={searchResult} />
        </span>
      )}
    </ResourceFields>
  );

  return (
    <IconAndContent
      Icon={icons.Application}
      iconColor="brand"
      iconOpacity={getRequestableResourceIconOpacity(props.searchResult)}
    >
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body2">
          {getAppItemCopy(
            $appName,
            app,
            searchResult.requiresRequest,
            props.isVnetSupported
          )}
        </Text>
        <Box ml="auto">
          <Text typography="body4">
            {props.getOptionalClusterName(app.uri)}
          </Text>
        </Box>
      </Flex>

      {/* If the description is long, put the resource fields on a separate line.
          Otherwise, show the resource fields and the labels together in a single line.
       */}
      {app.desc.length >= 30 ? (
        <>
          {$resourceFields}
          <Labels searchResult={searchResult} />
        </>
      ) : (
        <Labels searchResult={searchResult}>{$resourceFields}</Labels>
      )}
    </IconAndContent>
  );
}

function getAppItemCopy(
  $appName: React.JSX.Element,
  app: tsh.App,
  requiresRequest: boolean,
  isVnetSupported: boolean
) {
  if (requiresRequest) {
    return <>Request access to app {$appName}</>;
  }
  if (app.samlApp) {
    return <>Log in to {$appName} in the browser</>;
  }
  if (isWebApp(app) || app.awsConsole) {
    return <>Launch {$appName} in the browser</>;
  }

  // TCP app
  if (isVnetSupported) {
    return <>Connect with VNet to {$appName}</>;
  }
  return <>Set up an app connection to {$appName}</>;
}

export function KubeItem(props: SearchResultItem<SearchResultKube>) {
  const { searchResult } = props;

  return (
    <IconAndContent
      Icon={icons.Kubernetes}
      iconColor="brand"
      iconOpacity={getRequestableResourceIconOpacity(props.searchResult)}
    >
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body2">
          {props.searchResult.requiresRequest
            ? 'Request access to Kubernetes cluster '
            : 'Log in to Kubernetes cluster '}
          <strong>
            <HighlightField field="name" searchResult={searchResult} />
          </strong>
        </Text>
        <Box ml="auto">
          <Text typography="body4">
            {props.getOptionalClusterName(searchResult.resource.uri)}
          </Text>
        </Box>
      </Flex>

      <Labels searchResult={searchResult} />
    </IconAndContent>
  );
}

export function NoResultsItem(props: {
  clustersWithExpiredCerts: Set<uri.ClusterUri>;
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  advancedSearch: AdvancedSearch;
}) {
  const clustersWithExpiredCerts = Array.from(
    props.clustersWithExpiredCerts,
    clusterUri => props.getClusterName(clusterUri)
  );
  clustersWithExpiredCerts.sort();
  let expiredCertsCopy = '';

  if (clustersWithExpiredCerts.length === 1) {
    expiredCertsCopy = `The cluster ${clustersWithExpiredCerts[0]} was excluded from the search because you are not logged in to it.`;
  }

  if (clustersWithExpiredCerts.length > 1) {
    // prettier-ignore
    expiredCertsCopy = `The following clusters were excluded from the search because you are not logged in to them: ${clustersWithExpiredCerts.join(', ')}.`;
  }

  return (
    <NonInteractiveItem>
      <IconAndContent Icon={icons.Info} iconColor="text.slightlyMuted">
        <ContentAndAdvancedSearch advancedSearch={props.advancedSearch}>
          <Text typography="body2">No matching results found.</Text>
        </ContentAndAdvancedSearch>
        {expiredCertsCopy && <Text typography="body3">{expiredCertsCopy}</Text>}
      </IconAndContent>
    </NonInteractiveItem>
  );
}

export function TypeToSearchItem({
  hasNoRemainingFilterActions,
  advancedSearch,
}: {
  hasNoRemainingFilterActions: boolean;
  advancedSearch: AdvancedSearch;
}) {
  return (
    <NonInteractiveItem>
      <ContentAndAdvancedSearch advancedSearch={advancedSearch}>
        <Text typography="body3">
          Enter space-separated search terms.
          {hasNoRemainingFilterActions ||
            ' Select a filter to narrow down the search.'}
        </Text>
      </ContentAndAdvancedSearch>
    </NonInteractiveItem>
  );
}

export function AdvancedSearchEnabledItem({
  advancedSearch,
}: {
  advancedSearch: AdvancedSearch;
}) {
  return (
    <NonInteractiveItem>
      <ContentAndAdvancedSearch advancedSearch={advancedSearch}>
        <Text typography="body3">
          Enter the query using the predicate language. Inline results are not
          available in this mode.
        </Text>
      </ContentAndAdvancedSearch>
    </NonInteractiveItem>
  );
}

export function ResourceSearchErrorsItem(props: {
  errors: ResourceSearchError[];
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  showErrorsInModal: () => void;
  advancedSearch: AdvancedSearch;
}) {
  const { errors, getClusterName } = props;

  let shortDescription: string;

  if (errors.length === 1) {
    const firstErrorMessage = errors[0].messageWithClusterName(getClusterName);
    shortDescription = `${firstErrorMessage}.`;
  } else {
    const allErrorMessages = errors
      .map(err =>
        err.messageWithClusterName(getClusterName, { capitalize: false })
      )
      .join(', ');
    shortDescription = `Ran into ${errors.length} errors: ${allErrorMessages}.`;
  }

  return (
    <NonInteractiveItem>
      <IconAndContent Icon={icons.Warning} iconColor="warning.main">
        <ContentAndAdvancedSearch advancedSearch={props.advancedSearch}>
          <Text typography="body2">
            Some of the search results are incomplete.
          </Text>
        </ContentAndAdvancedSearch>

        <Flex gap={2} justifyContent="space-between" alignItems="baseline">
          <span
            css={`
              text-overflow: ellipsis;
              white-space: nowrap;
              overflow: hidden;
            `}
          >
            <Text typography="body3">{shortDescription}</Text>
          </span>

          <ButtonBorder
            type="button"
            size="small"
            css={`
              flex-shrink: 0;
            `}
            onClick={props.showErrorsInModal}
          >
            Show details
          </ButtonBorder>
        </Flex>
      </IconAndContent>
    </NonInteractiveItem>
  );
}

function Labels(
  props: React.PropsWithChildren<{
    searchResult: ResourceSearchResult;
  }>
) {
  const { searchResult } = props;

  // Label name to score.
  const scoreMap: Map<string, number> = new Map();
  searchResult.labelMatches.forEach(match => {
    const currentScore = scoreMap.get(match.labelName) || 0;
    scoreMap.set(match.labelName, currentScore + match.score);
  });

  const sortedLabelsList = [...searchResult.resource.labels];
  sortedLabelsList.sort(
    (a, b) =>
      // Highest score first.
      (scoreMap.get(b.name) || 0) - (scoreMap.get(a.name) || 0)
  );

  return (
    <LabelsFlex>
      {props.children}
      {sortedLabelsList.map(label => (
        <Label
          key={label.name + label.value}
          searchResult={searchResult}
          label={label}
        />
      ))}
    </LabelsFlex>
  );
}

const LabelsFlex = styled(Flex).attrs({ gap: 1 })`
  overflow-x: hidden;
  flex-wrap: nowrap;
  align-items: baseline;

  // Make the children not shrink, otherwise they would shrink in attempt to render all labels in
  // the same row.

  & > * {
    flex-shrink: 0;
  }
`;

const ResourceFields = styled(Flex).attrs({ gap: 1 })`
  color: ${props => props.theme.colors.text.main};
  font-size: ${props => props.theme.fontSizes[0]}px;
`;

function Label(props: {
  searchResult: ResourceSearchResult;
  label: tsh.Label;
}) {
  const { searchResult: item, label } = props;
  const labelMatches = item.labelMatches.filter(
    match => match.labelName == label.name
  );
  const nameMatches = labelMatches
    .filter(match => match.kind === 'label-name')
    .map(match => match.searchTerm);
  const valueMatches = labelMatches
    .filter(match => match.kind === 'label-value')
    .map(match => match.searchTerm);

  return (
    <DesignLabel
      key={label.name}
      kind="secondary"
      title={`${label.name}: ${label.value}`}
    >
      <Highlight text={label.name} keywords={nameMatches} />:{' '}
      <Highlight text={label.value} keywords={valueMatches} />
    </DesignLabel>
  );
}

function HighlightField(props: {
  searchResult: ResourceSearchResult;
  field: ResourceMatch<ResourceSearchResult['kind']>['field'];
}) {
  // `as` used as a workaround for a TypeScript issue.
  // https://github.com/microsoft/TypeScript/issues/33591
  const keywords = (
    props.searchResult.resourceMatches as ResourceMatch<
      ResourceSearchResult['kind']
    >[]
  )
    .filter(match => match.field === props.field)
    .map(match => match.searchTerm);

  return (
    <Highlight
      text={props.searchResult.resource[props.field]}
      keywords={keywords}
    />
  );
}

function FilterButton(props: { text: string; onClick(): void }) {
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      css={`
        color: ${props => props.theme.colors.buttons.text};
        background: ${props => props.theme.colors.spotBackground[1]};
        border-radius: ${props => props.theme.radii[2]}px;
      `}
      px="6px"
    >
      <CloseIcon
        color="buttons.text"
        mr={1}
        mt="1px"
        title="Remove filter"
        onClick={props.onClick}
        css={`
          cursor: pointer;
          border-radius: ${props => props.theme.radii[1]}px;

          &:hover {
            background: ${props => props.theme.colors.spotBackground[1]};
          }

          > svg {
            height: 13px;
            width: 13px;
          }
        `}
      />
      <span
        title={props.text}
        css={`
          white-space: nowrap;
          cursor: default;
        `}
      >
        {props.text}
      </span>
    </Flex>
  );
}

interface AdvancedSearch {
  isToggled: boolean;
  onToggle(): void;
}

function ContentAndAdvancedSearch(
  props: React.PropsWithChildren<{
    advancedSearch: AdvancedSearch | undefined;
  }>
) {
  return (
    <Flex gap={2} justifyContent="space-between" alignItems="flex-start">
      {props.children}
      {props.advancedSearch && (
        <AdvancedSearchToggle {...props.advancedSearch} />
      )}
    </Flex>
  );
}

function getRequestableResourceIconOpacity(args: { requiresRequest: boolean }) {
  // Unified resources use 0.5 opacity for the requestable resources.
  return args.requiresRequest ? 0.5 : 1;
}
