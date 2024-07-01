import React, { useState, useEffect, useMemo } from 'react';
import { Prompt, useHistory } from 'react-router';
import { Transition } from 'react-transition-group';
import styled from 'styled-components';
import {
  Indicator,
  Box,
  Flex,
  ButtonPrimary,
  ButtonSecondary,
  Text,
  ButtonIcon,
} from 'design';
import { StyledPanel } from 'design/DataTable/StyledTable';
import { StyledArrowBtn } from 'design/DataTable/Pager/StyledPager';
import {
  Info as InfoIcon,
  CircleArrowLeft,
  CircleArrowRight,
  Magnifier,
  ListAddCheck,
  ArrowLeft,
} from 'design/Icon';
import Select from 'shared/components/Select';
import Link from 'design/Link';
import { Info } from 'design/Alert';
import { getNumAddedResources } from 'shared/components/AccessRequests/Shared/utils';
import { ClusterDropdown } from 'shared/components/ClusterDropdown/ClusterDropdown';
import UnifiedSearchPanel from 'teleport/UnifiedResources/SearchPanel';
import {
  FilterKind,
  UnifiedResources,
} from 'shared/components/UnifiedResources';
import { HoverTooltip } from 'shared/components/ToolTip';
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
import { useContentMinWidthContext } from 'teleport/Main';

import { getSalesURL } from 'teleport/services/sales';
import cfg from 'teleport/config';

import {
  ResourceList,
  ResourceKind,
} from 'shared/components/AccessRequests/NewRequest';

import useTeleportE from 'e-teleport/useTeleportE';

import { RequestCheckout } from './RequestCheckout';
import { useNewRequest, State, getResourceId } from './useNewRequest';
import { AppRequestButton, RequestButton } from './RequestButton';

const agentOptions: ResourceOption[] = [
  // Order matters. On initial render
  // the last element in the options array
  // will be used. Which can either be 'resource' or 'role'.
  {
    value: 'resource',
    label: 'resources',
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

function NewRequest(props: State) {
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
    resourceRequestsDisabled,
    fetchStatus,
    onAgentLabelClick,
    selectedResource,
    addedResources,
    appsGrantedByUserGroup,
    userGroupFetchAttempt,
    addOrRemoveResource,
    customSort,
    nextPage,
    prevPage,
    updateResourceKind,
    clearAddedResources,
    dryRunAttempt,
    requestableRoles,
    addAllFetchAttempt,
    usage,
    fetchUsage,
    ctx,
  } = props;
  const { setEnforceMinWidth } = useContentMinWidthContext();
  const history = useHistory();
  const { preferences, updatePreferences } = useUser();
  const [clusterDropdownError, setClusterDropdownError] = useState('');

  const [showCheckout, setShowCheckout] = useState(false);
  // warningConfirm holds the next resource option that will be applied
  // after user agrees to the warning dialogue.
  const [warningConfirm, setWarningConfirm] = useState<ResourceOption>();

  // Role based access requests are only allowed in root cluster.
  const resourceOptions = useMemo(() => {
    let options = [...agentOptions];
    if (!isLeafCluster) {
      options.unshift(roleOption);
    }
    return options;
  }, [isLeafCluster]);

  // Load the last option which is either:
  //  - option role if at a root cluster
  //  - option node if at a leaf cluster
  const [currResourceOpt, setCurrResourceOpt] = useState(
    resourceOptions[resourceOptions.length - 1]
  );

  useEffect(() => {
    setEnforceMinWidth(false);

    return () => {
      setEnforceMinWidth(true);
    };
  }, []);

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
  const numAddedResources = getNumAddedResources(addedResources);

  const isResourceRequest = numAddedResources > 0;
  const isRoleList = currResourceOpt.value === 'role';

  const numAddedRoles = Object.keys(addedResources.role).length;

  const numTotalSelections = numAddedResources + numAddedRoles;

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

  const igsDisabled = !cfg.isLegacyEnterprise() && !cfg.isIgsEnabled;
  const limitReached = usage && usageLimitReached(usage);
  const limited = limitReached || igsDisabled;
  const requestStarted = getNumAddedResources(addedResources) > 0;

  return (
    <FeatureBox>
      <FeatureHeader>
        <Flex width="100%" alignItems="center" justifyContent="space-between">
          <Flex alignItems="center">
            <HoverTooltip tipContent="Back to Access Requests">
              <ButtonIcon
                onClick={() => history.push(cfg.getAccessRequestRoute())}
                mr={2}
              >
                <ArrowLeft size="medium" />
              </ButtonIcon>
            </HoverTooltip>
            <FeatureHeaderTitle>New Request</FeatureHeaderTitle>
          </Flex>

          {limited && (
            <Box>
              <ButtonLockedFeature event={CtaEvent.CTA_ACCESS_REQUESTS}>
                <Text color="buttons.primary.text">
                  Unlock Unlimited Access Requests with Teleport Identity
                </Text>
              </ButtonLockedFeature>
            </Box>
          )}
        </Flex>
      </FeatureHeader>
      {attempt.status === 'failed' &&
        attempt.statusText !== dryRunAttempt.statusText && (
          <ErrorMessage message={attempt.statusText} />
        )}
      {addAllFetchAttempt.status === 'failed' && (
        <ErrorMessage message={addAllFetchAttempt.statusText} />
      )}
      {clusterDropdownError && <ErrorMessage message={clusterDropdownError} />}
      {dryRunAttempt.status === 'failed' && selectedResource !== 'role' && (
        <Info>{dryRunAttempt.statusText}</Info>
      )}
      {usage && <UsageInfo {...usage} />}
      {!usage && igsDisabled && <LimitedInfo />}
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
      {dryRunAttempt.status === 'success' &&
        selectedResource === 'resource' && (
          <UnifiedResources
            bulkActions={[
              {
                key: 'add_to_resource',
                action: addSelectedResources,
                Icon: ListAddCheck,
                text: 'Add/remove from request',
              },
            ]}
            ClusterDropdown={
              <ClusterDropdown
                clusterLoader={ctx.clusterService}
                clusterId={clusterId}
                onError={setClusterDropdownError}
              />
            }
            resources={resources.map(resource => ({
              resource,
              ui: {
                ActionButton:
                  resource.kind === 'app' ? (
                    <AppRequestButton
                      agent={resource}
                      addedResources={addedResources}
                      addOrRemoveResource={addOrRemoveResource}
                    />
                  ) : (
                    <RequestButton
                      disabled={resourceRequestsDisabled}
                      isAgentAdded={Boolean(
                        addedResources[resource.kind][getResourceId(resource)]
                      )}
                      onClick={() =>
                        addOrRemoveResource(
                          resource.kind,
                          getResourceId(resource)
                        )
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
              <ResourceList
                agents={agents}
                selectedResource={selectedResource}
                customSort={customSort}
                onLabelClick={onAgentLabelClick}
                addedResources={addedResources}
                requestStarted={requestStarted}
                addOrRemoveResource={addOrRemoveResource}
                requestableRoles={requestableRoles}
                disableRows={fetchStatus === 'loading'}
              />
              {!isRoleList && (
                <StyledPanel>
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
            appsGrantedByUserGroup={appsGrantedByUserGroup}
            addedResources={addedResources}
            userGroupFetchAttempt={userGroupFetchAttempt}
            onClose={() => {
              setShowCheckout(false);
              fetchUsage();
            }}
            toggleResource={({ kind, id, name }) =>
              addOrRemoveResource(kind, id, name)
            }
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
`;

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
    <UsageNotice data-testid="usage-info">
      <InfoIcon color="info" px={3} />
      <Text typography="paragraph">
        {limitReached ? (
          <>
            Your cluster has reached its allocation of {usage.used} access
            requests per month, but{' '}
            <Link href={getSalesLink()} target="_blank">
              you can get unlimited access requests with Teleport Identity.
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

function LimitedInfo() {
  return (
    <UsageNotice data-testid="usage-info">
      <InfoIcon color="info" px={3} />
      <Text typography="paragraph">
        Your cluster has an allocation of{' '}
        {cfg.featureLimits.AccessRequestMonthlyRequestLimit} access requests per
        month.
      </Text>
    </UsageNotice>
  );
}

const UsageNotice = styled(Flex)`
  border: 2px solid ${({ theme }) => theme.colors.info};
  background-color: ${({ theme }) => theme.colors.notice.background};
  border-radius: 8px;
  padding: ${p => p.theme.space[4]}px 0;
  width: 100%;
  height: 44px;
  align-items: center;
  flex: 0 0 auto;
  margin-top: ${p => p.theme.space[2]}px;
  margin-bottom: ${p => p.theme.space[4]}px;
`;

function usageLimitReached({ limit, used }: { limit: number; used: number }) {
  return used >= limit;
}

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
