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
import { AppSubKind } from 'teleport/services/apps';

import useTeleportE from 'e-teleport/useTeleportE';
import {
  deepCopyResourceMap,
  getResourceId,
  useNewRequest,
  addOrRemoveIdentityCenterAssignments,
  requestItems,
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
    addOrRemoveResources,
    addedResources,
    clearAddedResources,
    setAddedResources,
    updateNamespacesForKubeCluster,
  } = useNewRequest(ctx);
  const {
    clearAttempt,
    createAttempt,
    numAddedResources,
    cancelCheckout,
    ...requestCheckout
  } = useRequestCheckout({
    ctx,
    isResourceRequest: true,
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
          addOrRemoveResources={addOrRemoveResources}
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
            addOrRemoveResources(
              requestItems(
                resource.kind,
                getResourceId(resource, clusterId),
                resourceName
              )
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

    data.forEach(({ resource }) => {
      if (
        resource.kind === 'app' &&
        resource.subKind === AppSubKind.AwsIcAccount
      ) {
        addOrRemoveIdentityCenterAssignments(
          resource.name,
          resource.permissionSets,
          newResources
        );
        delete newResources['app'][resource.name];
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
          <>
            {/* Add a div with the width of the checkout to adjust the page layout so that the checkout doesn't cover the resources. */}
            <Box
              css={`
                min-width: 450px;
                height: 100%;
                // Counteract the padding on the page so that this is aligned with the requestcheckout.
                margin-right: -${props => props.theme.space[4]}px;
              `}
            />
            <CheckoutWrapper>
              <RequestCheckout
                {...requestCheckout}
                clearAttempt={clearAttempt}
                createAttempt={createAttempt}
                toggleResource={({ kind, id, name }) =>
                  addOrRemoveResources(requestItems(kind, id, name))
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
                updateNamespacesForKubeCluster={updateNamespacesForKubeCluster}
              />
            </CheckoutWrapper>
          </>
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
  width: 100%;
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
