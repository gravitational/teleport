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

import { Box, Flex, ButtonPrimary, ButtonText, Text, ButtonIcon } from 'design';
import { ChevronDown } from 'design/Icon';
import { pluralize } from 'shared/utils/text';

import { RequestCheckout } from 'shared/components/AccessRequests/NewRequest';

import useAccessRequestCheckout from './useAccessRequestCheckout';
import { AssumedRolesBar } from './AssumedRolesBar';

export function RequestCheckoutSuccess({
  onClose,
  reset,
}: RequestCheckoutSuccessProps) {
  return (
    <Box textAlign="center">
      <ButtonPrimary
        mt={5}
        mb={3}
        width="100%"
        size="large"
        onClick={() => {
          reset();
          onClose();
        }}
      >
        Back to Listings
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

type RequestCheckoutSuccessProps = {
  onClose: () => void;
  reset: () => void;
};

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
    resourceRequestRoles,
    fetchResourceRolesAttempt,
    setSelectedResourceRequestRoles,
    clearCreateAttempt,
    data,
    suggestedReviewers,
    selectedReviewers,
    setSelectedReviewers,
    assumedRequests,
    requestedCount,
    goToRequestsList: reset, // have to pass through RequestCheckout because works differently on web
    setShowCheckout,
    maxDuration,
    setMaxDuration,
    dryRunResponse,
    requestTTL,
    setRequestTTL,
  } = useAccessRequestCheckout();

  return (
    <>
      {data.length > 0 && !isCollapsed() && (
        <Box
          p={3}
          css={`
            border-top: 1px solid
              ${props => props.theme.colors.spotBackground[1]};
          `}
        >
          <Flex justifyContent="space-between" alignItems="center">
            <Text typography="h4" bold>
              {data.length} {pluralize(data.length, 'Resource')} Selected
            </Text>
            <Flex gap={3}>
              <ButtonPrimary onClick={() => setShowCheckout(!showCheckout)}>
                Proceed to Request
              </ButtonPrimary>
              <ButtonIcon onClick={collapseBar}>
                <ChevronDown size="medium" />
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
          <RequestCheckout
            toggleResource={toggleResource}
            onClose={() => setShowCheckout(false)}
            transitionState={transitionState}
            SuccessComponent={RequestCheckoutSuccess}
            reset={reset}
            data={data}
            createAttempt={createRequestAttempt}
            resourceRequestRoles={resourceRequestRoles}
            fetchResourceRequestRolesAttempt={fetchResourceRolesAttempt}
            selectedResourceRequestRoles={selectedResourceRequestRoles}
            setSelectedResourceRequestRoles={setSelectedResourceRequestRoles}
            createRequest={createRequest}
            clearAttempt={clearCreateAttempt}
            reviewers={suggestedReviewers}
            selectedReviewers={selectedReviewers}
            setSelectedReviewers={setSelectedReviewers}
            requireReason={false}
            numRequestedResources={requestedCount}
            isResourceRequest={data[0]?.kind !== 'role'}
            fetchStatus={'loaded'}
            dryRunResponse={dryRunResponse}
            maxDuration={maxDuration}
            setMaxDuration={setMaxDuration}
            requestTTL={requestTTL}
            setRequestTTL={setRequestTTL}
          />
        )}
      </Transition>
    </>
  );
}
