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

import React from 'react';
import styled from 'styled-components';
import { Prompt } from 'react-router';
import { pluralize } from 'shared/utils/text';
import { Link } from 'react-router-dom';
import { AddCircle } from 'design/Icon';
import { Box, ButtonPrimary, ButtonText, Flex, Text } from 'design';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { FeatureBox } from 'teleport/components/Layout';
import { ClusterResources } from 'teleport/UnifiedResources/UnifiedResources';
import { UnifiedResource } from 'teleport/services/agents';
import { ResourceActionButton } from 'teleport/UnifiedResources/ResourceActionButton';
import cfg from 'teleport/config';
import { RequestCheckout } from 'shared/components/AccessRequests/NewRequest';
import { SharedUnifiedResource } from 'shared/components/UnifiedResources';

import useTeleportE from 'e-teleport/useTeleportE';
import {
  getResourceId,
  useNewRequest,
} from 'e-teleport/Workflow/NewRequest/useNewRequest';
import {
  AppRequestButton,
  RequestButton,
} from 'e-teleport/Workflow/NewRequest/RequestButton';
import { useRequestCheckout } from 'e-teleport/Workflow/NewRequest/useRequestCheckout';

export function UnifiedResourcesE() {
  const ctx = useTeleportE();
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const includeRequestable = cfg.ui.showResources === 'requestable';

  // TODO (avatus): extract the necessary parts of useNewRequest and useRequestCheckout
  // into a new hook that can be shared between web and Connect
  const {
    addOrRemoveResource,
    addedResources,
    numAddedResources,
    clearAddedResources,
    addResource,
    removeResource,
  } = useNewRequest(ctx);
  const { clearAttempt, createAttempt, ...requestCheckout } =
    useRequestCheckout({
      ctx,
      selectedResource: 'resource',
      addedResources,
      reset: clearAddedResources,
    });
  const showCheckout =
    numAddedResources > 0 || createAttempt.status === 'success';

  const getActionButton = (resource: UnifiedResource) => {
    const isAgentAdded =
      !!addedResources[resource.kind][getResourceId(resource)];
    // if we are currently making an access request, all buttons change to
    // add to request
    const showRequestButton = resource.requiresRequest || showCheckout;
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
          onClick={() =>
            addOrRemoveResource(resource.kind, getResourceId(resource))
          }
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
    const allAdded = data.every(
      ({ resource }) => addedResources[resource.kind][getResourceId(resource)]
    );
    data.forEach(({ resource }) => {
      if (allAdded) {
        return removeResource(resource.kind, getResourceId(resource));
      }
      addResource(resource.kind, getResourceId(resource));
    });
  }

  return (
    <FeatureBox px={4}>
      <Flex gap={4}>
        <ResizingResourceWrapper showCheckout={showCheckout}>
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
            includeRequestable={includeRequestable}
            showCheckout={showCheckout}
          />
        </ResizingResourceWrapper>
        {showCheckout && (
          <CheckoutWrapper>
            <RequestCheckout
              {...requestCheckout}
              clearAttempt={clearAttempt}
              createAttempt={createAttempt}
              toggleResource={addOrRemoveResource}
              reset={clearAddedResources}
              onClose={clearAttempt}
              isResourceRequest={true} // only resource requests happen from this page
              Header={() => (
                <Box mb={3}>
                  <Text typography="h4" color="text.main" bold>
                    {numAddedResources}{' '}
                    {pluralize(numAddedResources, 'Resource')} Selected
                  </Text>
                </Box>
              )}
              SuccessComponent={SuccessActionComponent}
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
  max-height: calc(100vh - ${props => props.theme.topBarHeight[2]}px);
  overflow-y: auto;
`;

const ResizingResourceWrapper = styled(Box)`
  width: ${props => (props.showCheckout ? 'calc(100vw - 514px)' : '100%')};
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
