/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { Transition } from 'react-transition-group';

import {
  Box,
  Flex,
  ButtonPrimary,
  ButtonText,
  Text,
  ButtonIcon,
  Label,
} from 'design';
import * as Icon from 'design/Icon';
import { pluralize } from 'shared/utils/text';

import { RequestCheckoutWithSlider } from 'shared/components/AccessRequests/NewRequest';

import useAccessRequestCheckout from './useAccessRequestCheckout';
import { AssumedRolesBar } from './AssumedRolesBar';

const MAX_RESOURCES_IN_BAR_TO_SHOW = 5;

function RequestCheckoutSuccess({
  onClose,
  goToRequests,
}: {
  onClose(): void;
  goToRequests(): void;
}) {
  return (
    <Box textAlign="center">
      <ButtonPrimary
        mt={5}
        mb={3}
        width="100%"
        size="large"
        onClick={() => {
          goToRequests();
          onClose();
        }}
      >
        See requests
      </ButtonPrimary>
      <ButtonText
        onClick={() => {
          onClose();
        }}
      >
        Make Another Request
      </ButtonText>
    </Box>
  );
}

export function AccessRequestCheckout() {
  const {
    showCheckout,
    isCollapsed,
    collapseBar,
    setHasExited,
    createRequestAttempt,
    toggleResource,
    selectedResourceRequestRoles,
    createRequest,
    reset,
    resourceRequestRoles,
    fetchResourceRolesAttempt,
    setSelectedResourceRequestRoles,
    clearCreateAttempt,
    data,
    shouldShowClusterNameColumn,
    selectedReviewers,
    setSelectedReviewers,
    assumedRequests,
    requestedCount,
    goToRequestsList,
    setShowCheckout,
    maxDuration,
    onMaxDurationChange,
    maxDurationOptions,
    dryRunResponse,
    pendingRequestTtl,
    setPendingRequestTtl,
    pendingRequestTtlOptions,
    startTime,
    onStartTimeChange,
  } = useAccessRequestCheckout();

  const isRoleRequest = data[0]?.kind === 'role';

  function closeCheckout() {
    setShowCheckout(false);
  }

  // We should rather detect how much space we have,
  // but for simplicity we only count items.
  const moreToShow = Math.max(data.length - MAX_RESOURCES_IN_BAR_TO_SHOW, 0);
  return (
    <>
      {data.length > 0 && !isCollapsed() && (
        <Box
          px={3}
          py={2}
          css={`
            border-top: 1px solid
              ${props => props.theme.colors.spotBackground[1]};
          `}
        >
          <Flex
            justifyContent="space-between"
            alignItems="center"
            css={`
              gap: ${props => props.theme.space[1]}px;
            `}
          >
            <Flex flexDirection="column" minWidth={0}>
              <Text mb={1}>
                {data.length}{' '}
                {pluralize(data.length, isRoleRequest ? 'role' : 'resource')}{' '}
                added to access request:
              </Text>
              <Flex gap={1} flexWrap="wrap">
                {data
                  .slice(0, MAX_RESOURCES_IN_BAR_TO_SHOW)
                  .map(c => {
                    let resource = {
                      name: c.name,
                      key: `${c.clusterName}-${c.kind}-${c.id}`,
                      Icon: undefined,
                    };
                    switch (c.kind) {
                      case 'app':
                        resource.Icon = Icon.Application;
                        break;
                      case 'node':
                        resource.Icon = Icon.Server;
                        break;
                      case 'db':
                        resource.Icon = Icon.Database;
                        break;
                      case 'kube_cluster':
                        resource.Icon = Icon.Kubernetes;
                        break;
                      case 'role':
                        break;
                      default:
                        c satisfies never;
                    }
                    return resource;
                  })
                  .map(c => (
                    <Label
                      kind="secondary"
                      key={c.key}
                      css={`
                        display: flex;
                        align-items: center;
                        min-width: 0;
                        gap: ${props => props.theme.space[1]}px;
                      `}
                    >
                      {c.Icon && <c.Icon size={15} />}
                      <span
                        css={`
                          text-overflow: ellipsis;
                          white-space: nowrap;
                          overflow: hidden;
                        `}
                      >
                        {c.name}
                      </span>
                    </Label>
                  ))}
                {!!moreToShow && (
                  <Label kind="secondary">+ {moreToShow} more</Label>
                )}
              </Flex>
            </Flex>
            <Flex gap={3}>
              <ButtonPrimary
                onClick={() => setShowCheckout(!showCheckout)}
                textTransform="none"
                css={`
                  white-space: nowrap;
                `}
              >
                Proceed to request
              </ButtonPrimary>
              <ButtonIcon onClick={collapseBar}>
                <Icon.ChevronDown size="medium" />
              </ButtonIcon>
            </Flex>
          </Flex>
        </Box>
      )}
      {assumedRequests.map(request => (
        <AssumedRolesBar key={request.id} assumedRolesRequest={request} />
      ))}
      <Transition
        in={showCheckout}
        onEntered={() => setHasExited(false)}
        onExited={() => setHasExited(true)}
        timeout={300}
        mountOnEnter
        unmountOnExit
      >
        {transitionState => (
          <RequestCheckoutWithSlider
            toggleResource={toggleResource}
            onClose={closeCheckout}
            transitionState={transitionState}
            SuccessComponent={() =>
              RequestCheckoutSuccess({
                onClose: closeCheckout,
                goToRequests: goToRequestsList,
              })
            }
            reset={reset}
            data={data}
            showClusterNameColumn={shouldShowClusterNameColumn}
            createAttempt={createRequestAttempt}
            resourceRequestRoles={resourceRequestRoles}
            fetchResourceRequestRolesAttempt={fetchResourceRolesAttempt}
            selectedResourceRequestRoles={selectedResourceRequestRoles}
            setSelectedResourceRequestRoles={setSelectedResourceRequestRoles}
            createRequest={createRequest}
            clearAttempt={clearCreateAttempt}
            selectedReviewers={selectedReviewers}
            setSelectedReviewers={setSelectedReviewers}
            requireReason={false}
            numRequestedResources={requestedCount}
            isResourceRequest={!isRoleRequest}
            fetchStatus={'loaded'}
            dryRunResponse={dryRunResponse}
            maxDuration={maxDuration}
            onMaxDurationChange={onMaxDurationChange}
            maxDurationOptions={maxDurationOptions}
            pendingRequestTtl={pendingRequestTtl}
            pendingRequestTtlOptions={pendingRequestTtlOptions}
            setPendingRequestTtl={setPendingRequestTtl}
            startTime={startTime}
            onStartTimeChange={onStartTimeChange}
          />
        )}
      </Transition>
    </>
  );
}
