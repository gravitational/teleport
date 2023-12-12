import React, { useState, useEffect, useMemo } from 'react';
import { Prompt } from 'react-router';
import { Transition } from 'react-transition-group';
import styled from 'styled-components';
import {
  Indicator,
  Box,
  Flex,
  ButtonPrimary,
  ButtonSecondary,
  Text,
  ButtonBorder,
  Button,
} from 'design';
import { kinds } from 'design/Button/Button';
import { StyledPanel } from 'design/DataTable/StyledTable';
import { StyledArrowBtn } from 'design/DataTable/Pager/StyledPager';
import {
  Info as InfoIcon,
  CircleArrowLeft,
  CircleArrowRight,
  Magnifier,
  ListAddCheck,
} from 'design/Icon';
import Select from 'shared/components/Select';
import Link from 'design/Link';
import { Info } from 'design/Alert';
import { SearchPanel } from 'shared/components/Search';
import { storageService } from 'teleport/services/storageService';
import { Attempt } from 'shared/hooks/useAttemptNext';
import UnifiedSearchPanel from 'teleport/UnifiedResources/SearchPanel';
import {
  FilterKind,
  UnifiedResources,
} from 'shared/components/UnifiedResources';
import { TextIcon } from 'teleport/Discover/Shared';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import ErrorMessage from 'teleport/components/AgentErrorMessage';
import { CtaEvent } from 'teleport/services/userEvent';
import { useUser } from 'teleport/User/UserContext';

import { getSalesURL } from 'teleport/services/sales';
import cfg from 'teleport/config';

import useTeleportE from 'e-teleport/useTeleportE';

import { ResourceList } from './ResourceList';
import { RequestCheckout } from './RequestCheckout';
import {
  useNewRequest,
  State,
  getResourceId,
  ResourceKind,
} from './useNewRequest';
import { RequestButton } from './RequestButton';

import type { TransitionStatus } from 'react-transition-group';

const agentOptions: ResourceOption[] = [
  {
    value: 'user_group',
    label: 'user groups',
  },
  // Order matters. On initial render
  // the last element in the options array
  // will be used. Which can either be 'resource' or 'role'.
  {
    value: 'resource',
    label: 'resources',
  },
];

const legacyAgentOptions: ResourceOption[] = [
  {
    value: 'app',
    label: 'applications',
  },
  {
    value: 'db',
    label: 'databases',
  },
  {
    value: 'windows_desktop',
    label: 'desktops',
  },
  {
    value: 'kube_cluster',
    label: 'kubernetes',
  },
  {
    value: 'user_group',
    label: 'user groups',
  },
  // Order matters. On initial render
  // the last element in the options array
  // will be used. Which can either be 'node' or 'role'.
  {
    value: 'node',
    label: 'servers',
  },
];

// roleOption is separate because role based
// and search based access requests cannot
// be combined.
const roleOption: ResourceOption = {
  value: 'role',
  label: 'roles',
};

export default function Container() {
  const teleCtx = useTeleportE();
  const state = useNewRequest(teleCtx);

  return <NewRequest {...state} />;
}

// we don't want to disable any kinds in the filter
// for access requests because we don't know what
// kinds the user might have the ability to request
const availableKinds: FilterKind[] = [
  {
    kind: 'node',
    disabled: false,
  },
  {
    kind: 'app',
    disabled: false,
  },
  {
    kind: 'db',
    disabled: false,
  },
  {
    kind: 'kube_cluster',
    disabled: false,
  },
  {
    kind: 'windows_desktop',
    disabled: false,
  },
];

export function NewRequest(props: State) {
  const {
    isLeafCluster,
    clusterId,
    attempt,
    agents,
    agentFilter,
    setAgentFilter,
    unifiedFetch,
    unifiedFetchAttempt,
    resources,
    addSelectedResources,
    updateQuery,
    resourceRequestsDisabled,
    updateSearch,
    fetchStatus,
    onAgentLabelClick,
    selectedResource,
    addedResources,
    addOrRemoveResource,
    pageCount,
    customSort,
    nextPage,
    prevPage,
    updateResourceKind,
    clearAddedResources,
    dryRunAttempt,
    requestableRoles,
    toggleAddCurrentPage,
    toggleAddAllPages,
    numOfPages,
    numAddedOnPage,
    addedAll,
    addAllFetchAttempt,
    usage,
    fetchUsage,
  } = props;
  const unifiedResourcesEnabled = storageService.areUnifiedResourcesEnabled();
  const { preferences, updatePreferences } = useUser();

  const [showCheckout, setShowCheckout] = useState(false);
  // warningConfirm holds the next resource option that will be applied
  // after user agrees to the warning dialogue.
  const [warningConfirm, setWarningConfirm] = useState<ResourceOption>();

  // Role based access requests are only allowed in root cluster.
  const resourceOptions = useMemo(() => {
    let options = unifiedResourcesEnabled
      ? [...agentOptions]
      : [...legacyAgentOptions];
    if (!isLeafCluster) {
      options.unshift(roleOption);
    }
    return options;
  }, [unifiedResourcesEnabled, isLeafCluster]);

  // Load the last option which is either:
  //  - option role if at a root cluster
  //  - option node if at a leaf cluster
  const [currResourceOpt, setCurrResourceOpt] = useState(
    resourceOptions[resourceOptions.length - 1]
  );

  useEffect(() => {
    if (dryRunAttempt.status === 'failed') {
      setCurrResourceOpt(roleOption);
      updateResourceKind('role');
    }
  }, [dryRunAttempt]);

  useEffect(() => {
    const newOption = resourceOptions[resourceOptions.length - 1];
    // if resourceOptions have changed, then unified support has changed.
    // We need to reset the selected resource
    setCurrResourceOpt(newOption);
    updateResourceKind(newOption.value);
  }, [resourceOptions, updateResourceKind]);

  // numAddedResources is the number of resources added to the Access Request without counting roles.
  // Having any of these resources added to the Access Request makes it a Resource Access Request
  const numAddedResources =
    Object.keys(addedResources.node).length +
    Object.keys(addedResources.db).length +
    Object.keys(addedResources.app).length +
    Object.keys(addedResources.kube_cluster).length +
    Object.keys(addedResources.user_group).length +
    Object.keys(addedResources.windows_desktop).length;

  const isResourceRequest = numAddedResources > 0;
  const isRoleList = currResourceOpt.value === 'role';

  const numAddedRoles = Object.keys(addedResources.role).length;

  const numTotalSelections = numAddedResources + numAddedRoles;

  const showAddAllPagesPanel =
    numAddedOnPage === agents.length && numOfPages > 1 && !isRoleList;

  // 'confirmed' parameter is only true when user agrees to the warning dialogue.
  function handleOnChangeResourceOption(o: ResourceOption, confirmed = false) {
    // Warn users when user is switching between search based requests (AgentKinds) and
    // role based requests when items were selected.
    if (
      !confirmed &&
      ((o.value === 'role' && numAddedResources > 0) ||
        (o.value !== 'role' && numAddedRoles > 0))
    ) {
      setWarningConfirm(o);
      return;
    }

    setCurrResourceOpt(o);
    updateResourceKind(o.value);
  }

  /* This is a warning prompt when user switches between role based and search based requests. */
  if (warningConfirm) {
    const msg = `Resource Access Request cannot be combined with Role Access Request. Current items selected will be cleared. Are you sure you want to continue?`;
    if (window.confirm(msg)) {
      clearAddedResources();
      handleOnChangeResourceOption(warningConfirm, true);
    }
    setWarningConfirm(null);
  }

  const limitReached = usage && usageLimitReached(usage);

  return (
    <FeatureBox>
      <FeatureHeader>
        <Flex width="100%" alignItems="center" justifyContent="space-between">
          <FeatureHeaderTitle>New Request</FeatureHeaderTitle>

          {limitReached && (
            <Box>
              <ButtonLockedFeature event={CtaEvent.CTA_ACCESS_REQUESTS}>
                <Text color="buttons.primary.text">
                  Unlock Unlimited Access Requests{' '}
                  {cfg.isTeam
                    ? 'with Teleport Enterprise'
                    : 'with Identity Governance & Security'}
                </Text>
              </ButtonLockedFeature>
            </Box>
          )}
        </Flex>
      </FeatureHeader>
      {attempt.status === 'failed' && (
        <ErrorMessage message={attempt.statusText} />
      )}
      {addAllFetchAttempt.status === 'failed' && (
        <ErrorMessage message={addAllFetchAttempt.statusText} />
      )}
      {dryRunAttempt.status === 'failed' && selectedResource !== 'role' && (
        <Info>{dryRunAttempt.statusText}</Info>
      )}
      {usage && <UsageInfo {...usage} />}
      <Flex justifyContent="space-between" alignItems="center" mb={4}>
        <Box width="150px" data-testid="resource-selector">
          <Select
            value={currResourceOpt}
            options={resourceOptions}
            onChange={o => handleOnChangeResourceOption(o as ResourceOption)}
            isDisabled={false}
            css={`
              text-transform: capitalize;
            `}
          />
        </Box>
        <Flex
          data-testid="checkout-footer"
          alignItems="center"
          justifyContent="space-between"
        >
          <Text mr={4} bold>
            Resources Added ({numTotalSelections})
          </Text>
          <Box>
            {numTotalSelections > 0 && (
              <ButtonSecondary
                mr={3}
                width="165px"
                onClick={clearAddedResources}
              >
                Clear Selections
              </ButtonSecondary>
            )}
            <ButtonPrimary
              width="182px"
              onClick={() => setShowCheckout(true)}
              disabled={numTotalSelections === 0}
            >
              Proceed to Request
            </ButtonPrimary>
          </Box>
        </Flex>
      </Flex>
      {dryRunAttempt.status === 'success' && selectedResource === 'resource' && (
        <UnifiedResources
          bulkActions={[
            {
              key: 'add_to_resource',
              action: addSelectedResources,
              Icon: ListAddCheck,
              text: 'Add/remove from request',
            },
          ]}
          resources={resources.map(resource => ({
            resource,
            ui: {
              ActionButton: (
                <RequestButton
                  disabled={resourceRequestsDisabled}
                  isAgentAdded={Boolean(
                    addedResources[resource.kind][getResourceId(resource)]
                  )}
                  toggleAgent={() =>
                    addOrRemoveResource(resource.kind, getResourceId(resource))
                  }
                />
              ),
            },
          }))}
          fetchResources={unifiedFetch}
          resourcesFetchAttempt={unifiedFetchAttempt}
          params={agentFilter}
          setParams={setAgentFilter}
          unifiedResourcePreferences={preferences.unifiedResourcePreferences}
          updateUnifiedResourcesPreferences={preferences => {
            updatePreferences({ unifiedResourcePreferences: preferences });
          }}
          pinning={{ kind: 'hidden' }}
          availableKinds={availableKinds}
          // we only use the SearchPanel in the header because we will need a separate
          // header that includes the request type dropdown and Proceed to Request button
          Header={
            <Flex justifyContent="space-between" alignItems="center">
              <UnifiedSearchPanel
                params={agentFilter}
                setParams={setAgentFilter}
                // the following two parameters aren't needed as we don't need url
                // filtering to work inside access requests so we can no-op them
                pathname={''}
                replaceHistory={() => {}}
              />
            </Flex>
          }
          key={clusterId}
          NoResources={
            <NoResults query={agentFilter?.query || agentFilter?.search} />
          }
        />
      )}
      {selectedResource !== 'resource' && (
        <Box>
          {(attempt.status === 'processing' ||
            dryRunAttempt.status === 'processing') && (
            <Box textAlign="center" m={10}>
              <Indicator />
            </Box>
          )}
          {attempt.status !== 'processing' && (
            <StyledWrapper>
              {/*roles use client-side search */}
              {!isRoleList && (
                <>
                  <SearchPanel
                    updateQuery={updateQuery}
                    updateSearch={updateSearch}
                    pageIndicators={pageCount}
                    filter={agentFilter}
                    showSearchBar={true}
                    disableSearch={fetchStatus === 'loading'}
                    extraChildren={
                      <AddPageButton
                        toggleAddCurrentPage={toggleAddCurrentPage}
                        toggleAddAllPages={toggleAddAllPages}
                        agentOption={currResourceOpt}
                        areAllPagesAdded={addedAll[currResourceOpt.value]}
                        numAdded={
                          addedAll[currResourceOpt.value]
                            ? Object.keys(addedResources[currResourceOpt.value])
                                .length
                            : numAddedOnPage
                        }
                      />
                    }
                  />
                  <Transition
                    in={showAddAllPagesPanel}
                    timeout={50}
                    mountOnEnter
                    unmountOnExit
                    enter
                    exit
                  >
                    {transitionState => (
                      <AddAllPagesPanel
                        toggleAddAllPages={toggleAddAllPages}
                        totalCount={pageCount.total}
                        pageCount={numOfPages}
                        areAllPagesAdded={addedAll[currResourceOpt.value]}
                        numAddedOnPage={numAddedOnPage}
                        agentOption={currResourceOpt}
                        attempt={addAllFetchAttempt}
                        transitionState={transitionState}
                      />
                    )}
                  </Transition>
                </>
              )}
              <ResourceList
                agents={agents}
                selectedResource={selectedResource}
                customSort={customSort}
                onLabelClick={onAgentLabelClick}
                addedResources={addedResources}
                addOrRemoveResource={addOrRemoveResource}
                requestableRoles={requestableRoles}
                disableRows={fetchStatus === 'loading'}
              />
              {!isRoleList && (
                <StyledPanel
                  borderBottomLeftRadius={3}
                  borderBottomRightRadius={3}
                  showTopBorder={true}
                >
                  <Flex justifyContent="flex-end" width="100%">
                    <Flex alignItems="center" mr={2}></Flex>
                    <Flex>
                      <StyledArrowBtn
                        onClick={prevPage}
                        title="Previous page"
                        disabled={!prevPage || fetchStatus === 'loading'}
                        mx={0}
                      >
                        <CircleArrowLeft />
                      </StyledArrowBtn>
                      <StyledArrowBtn
                        ml={0}
                        onClick={nextPage}
                        title="Next page"
                        disabled={!nextPage || fetchStatus === 'loading'}
                      >
                        <CircleArrowRight />
                      </StyledArrowBtn>
                    </Flex>
                  </Flex>
                </StyledPanel>
              )}
            </StyledWrapper>
          )}
        </Box>
      )}
      <Transition in={showCheckout} timeout={300} mountOnEnter unmountOnExit>
        {transitionState => (
          <RequestCheckout
            addedResources={addedResources}
            onClose={() => {
              setShowCheckout(false);
              fetchUsage();
            }}
            toggleResource={addOrRemoveResource}
            transitionState={transitionState}
            reset={clearAddedResources}
            selectedResource={selectedResource}
            isResourceRequest={isResourceRequest}
          />
        )}
      </Transition>
      {/* This is a react-router provided prompt when it detects route change.
       * Used when user navigates away or changes cluster (which changes the route).
       */}
      <Prompt
        when={numTotalSelections > 0}
        message={location => {
          if (location.pathname.endsWith('/requests/new')) {
            return `Resources from different clusters cannot be combined in an access request. Current items selected will be cleared. Are you sure you want to continue?`;
          } else {
            return `${numTotalSelections} item(s) selected for a new access request will be cleared if you leave this page. Are you sure you want to continue?`;
          }
        }}
      />
    </FeatureBox>
  );
}

const StyledWrapper = styled.div`
  border-radius: 8px;
  box-shadow: ${props => props.theme.boxShadow[0]};
`;

function AddPageButton({
  toggleAddCurrentPage,
  toggleAddAllPages,
  numAdded,
  areAllPagesAdded,
  agentOption,
}: {
  toggleAddCurrentPage: () => void;
  toggleAddAllPages: () => void;
  numAdded: number;
  areAllPagesAdded: boolean;
  agentOption: ResourceOption;
}) {
  let agentButtonText =
    agentOption.label === 'kubernetes' ? 'clusters' : agentOption.label;

  // Removes the 's' at the end if only one is selected
  if (numAdded === 1) {
    agentButtonText = agentButtonText.slice(0, -1);
  }

  function onClick() {
    if (areAllPagesAdded) {
      toggleAddAllPages();
    } else {
      toggleAddCurrentPage();
    }
  }

  return (
    <AnimatedButton
      ml={3}
      onClick={onClick}
      className={numAdded > 0 ? 'primary' : 'border'}
    >
      <Text className={numAdded > 0 ? 'primary' : 'border'}>
        {numAdded > 0 ? `Remove ${numAdded} ${agentButtonText}` : '+ Add all'}
      </Text>
    </AnimatedButton>
  );
}

function AddAllPagesPanel({
  toggleAddAllPages,
  totalCount,
  pageCount,
  numAddedOnPage,
  areAllPagesAdded,
  agentOption,
  attempt,
  transitionState,
}: {
  toggleAddAllPages: () => void;
  totalCount: number;
  pageCount: number;
  numAddedOnPage: number;
  areAllPagesAdded: boolean;
  agentOption: ResourceOption;
  attempt: Attempt;
  transitionState: TransitionStatus;
}) {
  const agentText =
    agentOption.label === 'kubernetes'
      ? 'kubernetes clusters'
      : agentOption.label;

  const agentButtonText =
    agentOption.label === 'kubernetes' ? 'clusters' : agentOption.label;

  if (areAllPagesAdded) {
    return (
      <>
        {attempt.status === 'success' && (
          <StyledSelectAllPanel className={transitionState}>
            <StyledSelectAllPanelContent className={transitionState}>
              <Text>
                All{' '}
                <Text as="span" bold>
                  {totalCount} {agentText} across {pageCount} pages
                </Text>{' '}
                have been added to your request.
              </Text>
              <ButtonPrimary ml={3} onClick={toggleAddAllPages} width="320px">
                Remove all {totalCount} matching {agentButtonText}
              </ButtonPrimary>
            </StyledSelectAllPanelContent>
          </StyledSelectAllPanel>
        )}
      </>
    );
  } else {
    return (
      <StyledSelectAllPanel className={transitionState}>
        <StyledSelectAllPanelContent className={transitionState}>
          <Text>
            All{' '}
            <Text as="span" bold>
              {numAddedOnPage} {agentText} on this page
            </Text>{' '}
            have been added to your request.
          </Text>
          <ButtonBorder
            ml={3}
            onClick={toggleAddAllPages}
            disabled={attempt.status === 'processing'}
            width="320px"
          >
            + Add all {totalCount} matching {agentButtonText}
          </ButtonBorder>
        </StyledSelectAllPanelContent>
      </StyledSelectAllPanel>
    );
  }
}

function UsageInfo(usage: { limit: number; used: number }) {
  const ctx = useTeleportE();
  // limit will be 0 if not using usage-based billing
  if (!usage.limit) {
    return null;
  }

  const limitReached = usageLimitReached(usage);

  // TODO
  // Does not emit event, like it automatically does when using ButtonLockedFeature component
  const getSalesLink = () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    return getSalesURL(version, cfg.isEnterprise, CtaEvent.CTA_ACCESS_REQUESTS);
  };

  return (
    <UsageNotice
      data-testid="usage-info"
      width="100%"
      height="44px"
      my={2}
      as={Flex}
      alignItems="center"
      flex="0 0 auto"
    >
      <InfoIcon color="info" px={3} />
      <Text typography="paragraph">
        {limitReached ? (
          <>
            Your cluster has reached its allocation of {usage.used} access
            requests per month, but{' '}
            <Link href={getSalesLink()} target="_blank">
              you can get unlimited access requests{' '}
              {cfg.isTeam
                ? 'with Teleport Enterprise'
                : 'with Identity Governance & Security'}
              .
            </Link>
          </>
        ) : (
          <>
            Your cluster has an allocation of {usage.limit} access requests per
            month. {usage.used}{' '}
            {usage.used == 1 ? 'access request has' : 'access requests have'}{' '}
            been created this month.
          </>
        )}
      </Text>
    </UsageNotice>
  );
}

const UsageNotice = styled(Box)`
  border: 2px solid ${({ theme }) => theme.colors.info};
  background-color: ${({ theme }) => theme.colors.notice.background};
  border-radius: 8px;
  margin-bottom: 24px;
  padding: 24px 0;
`;

function usageLimitReached({ limit, used }: { limit: number; used: number }) {
  return used >= limit;
}

const StyledSelectAllPanel = styled(StyledPanel)`
  justify-content: center;
  align-items: center;
  overflow: hidden;
  border-top: 2px solid ${props => props.theme.colors.spotBackground[0]};

  &.entering {
    height: 24px;
    padding-top: 16px;
    padding-bottom: 16px;
    transition: height 100ms ease-out, padding-top 100ms ease-out,
      padding-bottom 100ms ease-out;
  }
  &.entered {
    height: 24px;
    padding-top: 16px;
    padding-bottom: 16px;
    overflow: visible;
  }

  &.exiting {
    height: 0px;
    padding: 0px;
    padding-bottom: 0px;
    transition: height 50ms linear, padding-top 50ms linear,
      padding-bottom 50ms linear;
  }
  &.exited {
    height: 0px;
    padding-top: 0px;
    padding-bottom: 0px;
    border: none;
  }
`;

const StyledSelectAllPanelContent = styled(Flex)`
  align-items: center;
  justify-content: center;
  &.exiting,
  &.exited {
    display: none;
  }
`;

const ButtonBorderStyles = theme => ({ ...kinds({ kind: 'border', theme }) });
const ButtonPrimaryStyles = theme => ({ ...kinds({ kind: 'primary', theme }) });

const AnimatedButton = styled(Button)`
  white-space: nowrap;
  &.primary {
    width: 224px;
    transition: all 50ms ease-in;

    ${props => ButtonPrimaryStyles(props.theme)}
  }

  &.border {
    width: 120px;
    transition: all 50ms ease-in;

    ${props => ButtonBorderStyles(props.theme)}
  }
`;

type ResourceOption = {
  value: ResourceKind;
  label: string;
};

function NoResults({ query }: { query: string }) {
  // Prevent `No resources were found for ""` flicker.
  if (query) {
    return (
      <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
        <TextIcon typography="h3">
          <Magnifier />
          No resources were found for&nbsp;
          <Text
            as="span"
            bold
            css={`
              max-width: 270px;
              overflow: hidden;
              text-overflow: ellipsis;
            `}
          >
            {query}
          </Text>
        </TextIcon>
      </Box>
    );
  }
  return (
    <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
      <Text typography="h3">No requestable resources were found.</Text>
    </Box>
  );
}
