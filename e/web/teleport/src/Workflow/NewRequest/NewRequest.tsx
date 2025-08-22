import { useEffect, useRef, useState } from 'react';
import { Prompt, useHistory } from 'react-router';
import { Transition } from 'react-transition-group';
import styled from 'styled-components';

import {
  Box,
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  P1,
  Text,
} from 'design';
import { Info } from 'design/Alert';
import { FeatureName } from 'design/constants';
import {
  ArrowLeft,
  Info as InfoIcon,
  ListAddCheck,
  Magnifier,
} from 'design/Icon';
import Link from 'design/Link';
import { HoverTooltip } from 'design/Tooltip';
import Select from 'shared/components/Select';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  FilterKind,
  UnifiedResourceDefinition,
  UnifiedResources,
} from 'shared/components/UnifiedResources';
import {
  getResourceId as getUnifiedResourceId,
  openStatusInfoPanel,
} from 'shared/components/UnifiedResources/shared/StatusInfo';

import useTeleportE from 'e-teleport/useTeleportE';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { ClusterDropdown } from 'teleport/components/ClusterDropdown/ClusterDropdown';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { ServersideSearchPanel } from 'teleport/components/ServersideSearchPanel';
import cfg from 'teleport/config';
import { TextIcon } from 'teleport/Discover/Shared';
import { useNoMinWidth } from 'teleport/Main';
import { getSalesURL } from 'teleport/services/sales';
import { CtaEvent } from 'teleport/services/userEvent';
import { StatusInfo } from 'teleport/UnifiedResources/StatusInfo';
import { useUser } from 'teleport/User/UserContext';

import { AppRequestButton, RequestButton } from './RequestButton';
import { RequestCheckout } from './RequestCheckout';
import { Roles } from './Roles';
import {
  AccessRequestKind,
  getResourceId,
  requestItems,
  State,
  useNewRequest,
} from './useNewRequest';

const accessRequestTypeToLabel: Record<AccessRequestKind, string> = {
  resource: 'Resources',
  role: 'Roles',
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
    clusterId,
    agentFilter,
    setAgentFilter,
    unifiedFetch,
    unifiedFetchAttempt,
    resources,
    addSelectedResources,
    resourceRequestsDisabled,
    accessRequestKinds,
    selectedAccessRequestKind,
    addedResources,
    appsGrantedByUserGroup,
    userGroupFetchAttempt,
    addOrRemoveResources,
    updateAccessRequestKind,
    clearAddedResources,
    dryRunAttempt,
    fetchUsageAttempt,
    fetchUsage,
    ctx,
    updateNamespacesForKubeCluster,
    numAddedResources,
  } = props;
  useNoMinWidth();
  const history = useHistory();
  const { preferences, updatePreferences } = useUser();
  const [clusterDropdownError, setClusterDropdownError] = useState('');

  const [showCheckout, setShowCheckout] = useState(false);

  useEffect(() => {
    if (dryRunAttempt.status === 'failed') {
      updateAccessRequestKind('role');
    }
  }, [dryRunAttempt]);

  const isResourceRequest = numAddedResources > 0;
  const numAddedRoles = Object.keys(addedResources.role).length;
  const numTotalSelections = numAddedResources + numAddedRoles;

  function handleOnChangeResourceOption(o: AccessRequestKind) {
    // Warn when is switching between search-based requests and
    // role-based requests when items were selected.
    if (!numTotalSelections) {
      updateAccessRequestKind(o);
      return;
    }

    const msg = `Resource Access Request cannot be combined with Role Access Request. Current items selected will be cleared. Are you sure you want to continue?`;
    if (window.confirm(msg)) {
      clearAddedResources();
      updateAccessRequestKind(o);
    }
  }

  const { setInfoGuideConfig } = useInfoGuide();
  function onShowStatusInfo(resource: UnifiedResourceDefinition) {
    openStatusInfoPanel({
      resource,
      setInfoGuideConfig,
      guide: (
        <StatusInfo
          resource={resource}
          clusterId={clusterId}
          key={getUnifiedResourceId(resource)}
        />
      ),
    });
  }

  const limited =
    (fetchUsageAttempt.status === 'success' &&
      fetchUsageAttempt.data?.limit > 0) ||
    cfg.entitlements.AccessRequests.limit > 0;

  const transitionRef = useRef<HTMLDivElement>(null);

  const accessRequestOptions = accessRequestKinds.map(r => ({
    value: r,
    label: accessRequestTypeToLabel[r],
  }));

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
                  Unlock Unlimited Access Requests with{' '}
                  {FeatureName.IdentityGovernance}
                </Text>
              </ButtonLockedFeature>
            </Box>
          )}
        </Flex>
      </FeatureHeader>
      {fetchUsageAttempt.status === 'error' &&
        fetchUsageAttempt.statusText !== dryRunAttempt.statusText && (
          <ErrorMessage message={fetchUsageAttempt.statusText} />
        )}
      {clusterDropdownError && <ErrorMessage message={clusterDropdownError} />}
      {dryRunAttempt.status === 'failed' &&
        selectedAccessRequestKind !== 'role' && (
          <Info>{dryRunAttempt.statusText}</Info>
        )}

      <UsageInfo
        used={
          fetchUsageAttempt.status === 'success' && fetchUsageAttempt.data?.used
        }
        limit={cfg.entitlements.AccessRequests.limit}
      />

      <Flex justifyContent="space-between" alignItems="center" mb={4}>
        <Box width="150px" data-testid="resource-selector">
          <Select
            value={{
              value: selectedAccessRequestKind,
              label: accessRequestTypeToLabel[selectedAccessRequestKind],
            }}
            options={accessRequestOptions}
            onChange={o => handleOnChangeResourceOption(o.value)}
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
        selectedAccessRequestKind === 'resource' && (
          <UnifiedResources
            onShowStatusInfo={onShowStatusInfo}
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
                      addOrRemoveResources={addOrRemoveResources}
                    />
                  ) : (
                    <RequestButton
                      disabled={resourceRequestsDisabled}
                      isAgentAdded={Boolean(
                        addedResources[resource.kind][
                          getResourceId(resource, clusterId)
                        ]
                      )}
                      onClick={() => {
                        let resourceName;
                        if (resource.kind === 'node') {
                          resourceName = resource.hostname;
                        }
                        addOrRemoveResources(
                          requestItems(
                            resource.kind,
                            getResourceId(resource, clusterId),
                            resourceName
                          )
                        );
                      }}
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
              <Flex justifyContent="space-between" alignItems="center" mb={3}>
                <ServersideSearchPanel
                  params={agentFilter}
                  setParams={setAgentFilter}
                />
              </Flex>
            }
            key={clusterId}
            NoResources={
              <NoResults query={agentFilter?.query || agentFilter?.search} />
            }
          />
        )}
      {selectedAccessRequestKind === 'role' && (
        <Roles
          fetchFunc={ctx.resourceService.fetchRequestableRoles}
          allRequestableRoles={ctx.storeUser.getRequestableRoles()}
          requested={new Set(Object.keys(addedResources.role))}
          onToggleRole={role =>
            addOrRemoveResources(requestItems('role', role))
          }
        />
      )}
      <Transition
        in={showCheckout}
        nodeRef={transitionRef}
        timeout={300}
        mountOnEnter
        unmountOnExit
      >
        {transitionState => (
          <RequestCheckout
            ref={transitionRef}
            appsGrantedByUserGroup={appsGrantedByUserGroup}
            addedResources={addedResources}
            userGroupFetchAttempt={userGroupFetchAttempt}
            onClose={() => {
              setShowCheckout(false);
              void fetchUsage();
            }}
            toggleResource={({ kind, id, name }) =>
              addOrRemoveResources(requestItems(kind, id, name))
            }
            toggleResources={addOrRemoveResources}
            transitionState={transitionState}
            reset={clearAddedResources}
            isResourceRequest={isResourceRequest}
            updateNamespacesForKubeCluster={updateNamespacesForKubeCluster}
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

function UsageInfo(usage: { limit: number; used?: number }) {
  const ctx = useTeleportE();
  // limit will be 0 if not using usage-based billing
  if (!usage.limit) {
    return null;
  }

  // TODO(kimlisa)
  // Does not emit event, like it automatically does when using ButtonLockedFeature component
  const getSalesLink = () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    return getSalesURL(version, cfg.isEnterprise, CtaEvent.CTA_ACCESS_REQUESTS);
  };

  return (
    <UsageNotice data-testid="usage-info">
      <InfoIcon color="info" px={3} />
      <P1>
        {usage.used && usage.used >= usage.limit ? (
          <>
            Your cluster has reached its allocation of {usage.limit} access
            requests per month, but{' '}
            <Link href={getSalesLink()} target="_blank">
              you can get unlimited access requests with{' '}
              {FeatureName.IdentityGovernance}
            </Link>
          </>
        ) : (
          <>
            <>
              Your cluster has an allocation of {usage.limit} access requests
              per month.
            </>
            {usage.used && (
              <>
                {' '}
                {usage.used}{' '}
                {usage.used == 1
                  ? 'access request has'
                  : 'access requests have'}{' '}
                been created this month.
              </>
            )}
          </>
        )}
      </P1>
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

function NoResults({ query }: { query: string }) {
  // Prevent `No resources were found for ""` flicker.
  if (query) {
    return (
      <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
        <TextIcon typography="h1">
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
      <Text typography="h1">No requestable resources were found.</Text>
    </Box>
  );
}
