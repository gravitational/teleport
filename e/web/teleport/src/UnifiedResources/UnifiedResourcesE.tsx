import React from 'react';
import styled from 'styled-components';
import { Prompt } from 'react-router';
import { pluralize } from 'shared/utils/text';
import { Link } from 'react-router-dom';
import { AddCircle } from 'design/Icon';
import { Box, ButtonPrimary, ButtonText, Flex, H2 } from 'design';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { useUser } from 'teleport/User/UserContext';
import { FeatureBox } from 'teleport/components/Layout';
import { ClusterResources } from 'teleport/UnifiedResources/UnifiedResources';
import { UnifiedResource } from 'teleport/services/agents';
import { ResourceActionButton } from 'teleport/UnifiedResources/ResourceActionButton';
import cfg from 'teleport/config';
import {
  RequestCheckout,
  ResourceMap,
} from 'shared/components/AccessRequests/NewRequest';
import {
  IncludedResourceMode,
  SharedUnifiedResource,
  getResourceAvailabilityFilter,
} from 'shared/components/UnifiedResources';

import useTeleportE from 'e-teleport/useTeleportE';
import {
  deepCopyResourceMap,
  getResourceId,
  useNewRequest,
} from 'e-teleport/Workflow/NewRequest/useNewRequest';
import {
  AppRequestButton,
  RequestButton,
} from 'e-teleport/Workflow/NewRequest/RequestButton';
import { SamlAppEditAndDelete } from 'e-teleport/Discover/SamlApplication/EditAndDelete';
import { useRequestCheckout } from 'e-teleport/Workflow/NewRequest/useRequestCheckout';
import { SamlAppActionProvider } from 'e-teleport/SamlApplication/hooks/useSamlAppActionsE';

export function UnifiedResourcesE() {
  const ctx = useTeleportE();
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const includeRequestable = cfg.ui.showResources === 'requestable';

  // TODO (avatus): extract the necessary parts of useNewRequest and useRequestCheckout
  // into a new hook that can be shared between web and Connect
  const {
    addOrRemoveResource,
    addedResources,
    clearAddedResources,
    setAddedResources,
    bulkToggleResources,
  } = useNewRequest(ctx);
  const {
    clearAttempt,
    createAttempt,
    numAddedResources,
    cancelCheckout,
    ...requestCheckout
  } = useRequestCheckout({
    ctx,
    selectedResource: 'resource',
    addedResources,
    reset: clearAddedResources,
  });
  const showCheckout =
    numAddedResources > 0 || createAttempt.status === 'success';

  const getActionButton = (
    resource: UnifiedResource,
    includedResourceMode: IncludedResourceMode
  ) => {
    const isAgentAdded =
      !!addedResources[resource.kind][getResourceId(resource, clusterId)];
    // if we are currently making an access request, all buttons change to
    // add to request
    const showRequestButton =
      resource.requiresRequest ||
      showCheckout ||
      includedResourceMode === 'requestable';
    const requestStarted = numAddedResources > 0;

    if (showRequestButton && resource.kind === 'app') {
      return (
        <AppRequestButton
          agent={resource}
          addOrRemoveResource={addOrRemoveResource}
          addedResources={addedResources}
          requestStarted={requestStarted}
        />
      );
    }

    if (showRequestButton) {
      return (
        <RequestButton
          isAgentAdded={isAgentAdded}
          onClick={() => {
            let resourceName;
            if (resource.kind === 'node') {
              resourceName = resource.hostname;
            }
            addOrRemoveResource(
              resource.kind,
              getResourceId(resource, clusterId),
              resourceName
            );
          }}
          disabled={false}
          requestStarted={requestStarted}
        />
      );
    }
    return <ResourceActionButton resource={resource} />;
  };

  function bulkAdd(
    data: {
      resource: SharedUnifiedResource['resource'];
    }[]
  ) {
    const newResources: ResourceMap = deepCopyResourceMap(addedResources);
    const allAdded = data.every(
      ({ resource }) =>
        newResources[resource.kind][getResourceId(resource, clusterId)]
    );
    data.forEach(({ resource }) => {
      const resourceId = getResourceId(resource, clusterId);
      const resourceName =
        resource.kind === 'node' ? resource.hostname : resourceId;
      if (allAdded) {
        delete newResources[resource.kind][resourceId];
      } else {
        newResources[resource.kind][resourceId] = resourceName;
      }
    });
    setAddedResources(newResources);
  }

  const { preferences } = useUser();

  const availabilityFilterFromPreferences = getResourceAvailabilityFilter(
    preferences?.unifiedResourcePreferences?.availableResourceMode,
    cfg.ui.showResources === 'requestable'
  );

  return (
    <FeatureBox px={4}>
      <Flex gap={4}>
        <ResizingResourceWrapper showCheckout={showCheckout}>
          <SamlAppActionProvider>
            <SamlAppEditAndDelete />
            <ClusterResources
              bulkActions={
                includeRequestable
                  ? [
                      {
                        key: 'requestAccess',
                        Icon: AddCircle,
                        text:
                          numAddedResources > 0
                            ? 'Add/Remove to Request'
                            : 'Request Access',
                        disabled: false,
                        action: bulkAdd,
                      },
                    ]
                  : []
              }
              key={clusterId} // when the current cluster changes, remount the component
              clusterId={clusterId}
              isLeafCluster={isLeafCluster}
              getActionButton={getActionButton}
              availabilityFilter={
                availabilityFilterFromPreferences.canRequestAll
                  ? availabilityFilterFromPreferences
                  : null
              }
              showCheckout={showCheckout}
            />
          </SamlAppActionProvider>
        </ResizingResourceWrapper>
        {showCheckout && (
          <CheckoutWrapper>
            <RequestCheckout
              {...requestCheckout}
              clearAttempt={clearAttempt}
              createAttempt={createAttempt}
              toggleResource={({ kind, id, name }) =>
                addOrRemoveResource(kind, id, name)
              }
              reset={cancelCheckout}
              onClose={clearAttempt}
              isResourceRequest={true} // only resource requests happen from this page
              Header={() => (
                <Box mb={3}>
                  <H2>
                    New Access Request: {numAddedResources}{' '}
                    {pluralize(numAddedResources, 'Resource')} Selected
                  </H2>
                </Box>
              )}
              SuccessComponent={SuccessActionComponent}
              bulkToggleKubeResources={items => bulkToggleResources(items)}
            />
          </CheckoutWrapper>
        )}
        <Prompt
          when={numAddedResources > 0}
          message={location => {
            if (location.pathname.endsWith('/resources')) {
              return `Resources from different clusters cannot be combined in an access request. Current items selected will be cleared. Are you sure you want to continue?`;
            } else {
              return `${numAddedResources} item(s) selected for a new access request will be cleared if you leave this page. Are you sure you want to continue?`;
            }
          }}
        />
      </Flex>
    </FeatureBox>
  );
}

const CheckoutWrapper = styled(Box)`
  width: 450px;
  position: absolute;
  right: ${props => props.theme.space[3]}px; // avoid covering the scrollbar
  padding: ${props => props.theme.space[5]}px;
  padding-bottom: 0px;
  background-color: ${props => props.theme.colors.levels.sunken};
  max-height: calc(100vh - ${props => props.theme.topBarHeight[1]}px);
  overflow-y: auto;
`;

const ResizingResourceWrapper = styled(Box)<{ showCheckout?: boolean }>`
  width: ${props =>
    props.showCheckout ? 'calc(100vw - 514px - var(--sidenav-width))' : '100%'};
  padding-right: ${props => props.theme.space[3]}px;
`;

function SuccessActionComponent({ reset, onClose }) {
  return (
    <Box textAlign="center">
      <ButtonPrimary
        as={Link}
        mt={5}
        mb={3}
        width="100%"
        size="large"
        to={cfg.getAccessRequestRoute()}
      >
        View Requests
      </ButtonPrimary>
      <ButtonText
        onClick={() => {
          reset();
          onClose();
        }}
      >
        Close
      </ButtonText>
    </Box>
  );
}
