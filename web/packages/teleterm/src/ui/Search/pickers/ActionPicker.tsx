/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { ReactElement, useCallback, useMemo } from 'react';
import styled from 'styled-components';
import {
  Box,
  ButtonBorder,
  ButtonPrimary,
  Flex,
  Label as DesignLabel,
  Text,
} from 'design';
import * as icons from 'design/Icon';
import { Highlight } from 'shared/components/Highlight';
import { Attempt, hasFinished } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  ClusterSearchFilter,
  ResourceMatch,
  SearchResult,
  ResourceSearchResult,
  SearchResultDatabase,
  SearchResultKube,
  SearchResultServer,
  SearchResultCluster,
  SearchResultResourceType,
  SearchFilter,
} from 'teleterm/ui/Search/searchResult';
import * as tsh from 'teleterm/services/tshd/types';
import * as uri from 'teleterm/ui/uri';
import { ResourceSearchError } from 'teleterm/ui/services/resources';
import { isRetryable } from 'teleterm/ui/utils/retryWithRelogin';
import { assertUnreachable } from 'teleterm/ui/utils';

import { SearchAction } from '../actions';
import { useSearchContext } from '../SearchContext';
import { CrossClusterResourceSearchResult } from '../useSearch';

import { useActionAttempts } from './useActionAttempts';
import { getParameterPicker } from './pickers';
import { ResultList, NonInteractiveItem, IconAndContent } from './ResultList';
import { PickerContainer } from './PickerContainer';

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
  } = useSearchContext();
  const {
    filterActionsAttempt,
    resourceActionsAttempt,
    resourceSearchAttempt,
  } = useActionAttempts();
  const totalCountOfClusters = clustersService.getClusters().length;
  // The order of attempts is important. Filter actions should be displayed before resource actions.
  const actionAttempts = useMemo(
    () => [filterActionsAttempt, resourceActionsAttempt],
    [filterActionsAttempt, resourceActionsAttempt]
  );

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
        resetInput();
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
          key="resource-type"
          text={s.resourceType}
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
        filterActionsAttempt,
        actionAttempts,
        resourceSearchAttempt,
        allClusters: clustersService.getClusters(),
      }),
    [
      inputValue,
      filters,
      filterActionsAttempt,
      actionAttempts,
      resourceSearchAttempt,
      clustersService,
    ]
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

  return (
    <PickerContainer>
      <InputWrapper onKeyDown={handleKeyDown}>
        {filterButtons}
        {props.input}
      </InputWrapper>
      <ResultList<SearchAction>
        attempts={actionAttempts}
        onPick={onPick}
        onBack={close}
        addWindowEventListener={addWindowEventListener}
        render={item => {
          const Component = ComponentMap[item.searchResult.kind];
          return {
            key:
              item.searchResult.kind !== 'resource-type-filter'
                ? item.searchResult.resource.uri
                : item.searchResult.resource,
            Component: (
              <Component
                searchResult={item.searchResult}
                getOptionalClusterName={getOptionalClusterName}
              />
            ),
          };
        }}
        ExtraTopComponent={
          <ExtraTopComponents
            status={actionPickerStatus}
            getClusterName={getClusterName}
            showErrorsInModal={showErrorsInModal}
          />
        }
      />
    </PickerContainer>
  );
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
}) => {
  const { status, getClusterName, showErrorsInModal } = props;

  switch (status.inputState) {
    case 'no-input': {
      switch (status.searchMode.kind) {
        case 'no-search': {
          return <TypeToSearchItem hasNoRemainingFilterActions={false} />;
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
              />
              {nonRetryableResourceSearchErrors.length > 0 && (
                <ResourceSearchErrorsItem
                  errors={nonRetryableResourceSearchErrors}
                  getClusterName={getClusterName}
                  showErrorsInModal={() => {
                    showErrorsInModal(nonRetryableResourceSearchErrors);
                  }}
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
      return (
        <>
          {status.nonRetryableResourceSearchErrors.length > 0 && (
            <ResourceSearchErrorsItem
              errors={status.nonRetryableResourceSearchErrors}
              getClusterName={getClusterName}
              showErrorsInModal={() => {
                showErrorsInModal(status.nonRetryableResourceSearchErrors);
              }}
            />
          )}
          {status.hasNoResults && (
            <NoResultsItem
              clustersWithExpiredCerts={status.clustersWithExpiredCerts}
              getClusterName={getClusterName}
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
  filterActionsAttempt,
  allClusters,
  actionAttempts,
  resourceSearchAttempt,
}: {
  inputValue: string;
  filters: SearchFilter[];
  filterActionsAttempt: Attempt<SearchAction[]>;
  allClusters: tsh.Cluster[];
  actionAttempts: Attempt<SearchAction[]>[];
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
    const hasNoRemainingFilterActions =
      filterActionsAttempt.status === 'success' &&
      filterActionsAttempt.data.length === 0;

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
  const haveActionAttemptsFinished = actionAttempts.every(attempt =>
    hasFinished(attempt)
  );

  if (!haveActionAttemptsFinished) {
    return {
      inputState: 'some-input',
      hasNoResults: false,
      nonRetryableResourceSearchErrors,
      clustersWithExpiredCerts,
    };
  }

  const hasNoResults = actionAttempts.every(
    attempt => attempt.data.length === 0
  );

  // We could assume that resourceSearchAttempt has finished since action attempts depend on it and
  // we know that they all finished at this point. But we check status explicitly anyway.
  if (resourceSearchAttempt.status === 'success') {
    resourceSearchAttempt.data.errors.forEach(err => {
      if (isRetryable(err.cause)) {
        clustersWithExpiredCerts.add(err.clusterUri);
      } else {
        nonRetryableResourceSearchErrors.push(err);
      }
    });
  }

  // Make sure we don't list extra clusters with expired certs if a cluster filter is selected.
  const clusterFilter = filters.find(
    filter => filter.filter === 'cluster'
  ) as ClusterSearchFilter;
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
  'cluster-filter': ClusterFilterItem,
  'resource-type-filter': ResourceTypeFilterItem,
};

type SearchResultItem<T> = {
  searchResult: T;
  getOptionalClusterName: (uri: uri.ResourceUri) => string;
};

function ClusterFilterItem(props: SearchResultItem<SearchResultCluster>) {
  return (
    <IconAndContent Icon={icons.Lan} iconColor="text.slightlyMuted">
      <Text typography="body1">
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

const resourceIcons: Record<
  SearchResultResourceType['resource'],
  React.ComponentType<{
    color: string;
    fontSize: string;
    lineHeight: string;
  }>
> = {
  kubes: icons.Kubernetes,
  servers: icons.Server,
  databases: icons.Database,
};

function ResourceTypeFilterItem(
  props: SearchResultItem<SearchResultResourceType>
) {
  return (
    <IconAndContent
      Icon={resourceIcons[props.searchResult.resource]}
      iconColor="text.slightlyMuted"
    >
      <Text typography="body1">
        Search only for{' '}
        <strong>
          <Highlight
            text={props.searchResult.resource}
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
    <IconAndContent Icon={icons.Server} iconColor="brand">
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body1">
          Connect over SSH to{' '}
          <strong>
            <HighlightField field="hostname" searchResult={searchResult} />
          </strong>
        </Text>
        <Box ml="auto">
          <Text typography="body2" fontSize={0}>
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
    <IconAndContent Icon={icons.Database} iconColor="brand">
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body1">
          Set up a db connection to{' '}
          <strong>
            <HighlightField field="name" searchResult={searchResult} />
          </strong>
        </Text>
        <Box ml="auto">
          <Text typography="body2" fontSize={0}>
            {props.getOptionalClusterName(db.uri)}
          </Text>
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

export function KubeItem(props: SearchResultItem<SearchResultKube>) {
  const { searchResult } = props;

  return (
    <IconAndContent Icon={icons.Kubernetes} iconColor="brand">
      <Flex
        justifyContent="space-between"
        alignItems="center"
        flexWrap="wrap"
        gap={1}
      >
        <Text typography="body1">
          Log in to Kubernetes cluster{' '}
          <strong>
            <HighlightField field="name" searchResult={searchResult} />
          </strong>
        </Text>
        <Box ml="auto">
          <Text typography="body2" fontSize={0}>
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
        <Text typography="body1">No matching results found.</Text>
        {expiredCertsCopy && <Text typography="body2">{expiredCertsCopy}</Text>}
      </IconAndContent>
    </NonInteractiveItem>
  );
}

export function TypeToSearchItem({
  hasNoRemainingFilterActions,
}: {
  hasNoRemainingFilterActions: boolean;
}) {
  return (
    <NonInteractiveItem>
      <Text typography="body2">
        Enter space-separated search terms.
        {hasNoRemainingFilterActions ||
          ' Select a filter to narrow down the search.'}
      </Text>
    </NonInteractiveItem>
  );
}

export function ResourceSearchErrorsItem(props: {
  errors: ResourceSearchError[];
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  showErrorsInModal: () => void;
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
        <Text typography="body1">
          Some of the search results are incomplete.
        </Text>

        <Flex gap={2} justifyContent="space-between" alignItems="baseline">
          <span
            css={`
              text-overflow: ellipsis;
              white-space: nowrap;
              overflow: hidden;
            `}
          >
            <Text typography="body2">{shortDescription}</Text>
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

  const sortedLabelsList = [...searchResult.resource.labelsList];
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
    <ButtonPrimary
      px={2}
      size="small"
      title={props.text}
      onClick={props.onClick}
    >
      <span
        css={`
          max-width: calc(${props => props.theme.space[9]}px * 2);
          text-overflow: ellipsis;
          white-space: nowrap;
          overflow: hidden;
        `}
      >
        {props.text}
      </span>
    </ButtonPrimary>
  );
}
