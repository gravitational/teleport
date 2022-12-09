import React from 'react';
import styled from 'styled-components';
import { Transition } from 'react-transition-group';

import { Box, Flex, ButtonPrimary, ButtonText, Text } from 'design';
import { ArrowDown } from 'design/Icon';

import { pluralize } from 'teleport/lib/util';

import { RequestCheckout } from 'e-teleport/Workflow/NewRequest/RequestCheckout/RequestCheckout';

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
    assumedRequests,
    requestedCount,
    goToRequestsList: reset, // have to pass through RequestCheckout because works differently on web
    setShowCheckout,
  } = useAccessRequestCheckout();

  return (
    <>
      {data.length > 0 && !isCollapsed() && (
        <Box p={3} bg="primary.darker" border={1} borderColor="primary.dark">
          <Flex justifyContent="space-between" alignItems="center">
            <Text typography="h4" color="light" bold>
              {data.length} {pluralize(data.length, 'Resource')} Selected
            </Text>
            <Flex gap={3}>
              <ButtonPrimary onClick={() => setShowCheckout(!showCheckout)}>
                Proceed to Request
              </ButtonPrimary>
              <CollapseButton onClick={collapseBar}>
                <ArrowDown fontSize={3} />
              </CollapseButton>
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
            reviewers={[]} // need a way to get reviewers as options. for now you can create a text entry
            requireReason={false}
            numRequestedResources={requestedCount}
            isResourceRequest={data[0]?.kind !== 'role'}
          />
        )}
      </Transition>
    </>
  );
}

const CollapseButton = styled(Flex)`
  background: ${props => props.theme.colors.primary.dark};
  width: 26px;
  justify-content: center;
  align-items: center;
  height: 26px;
  border-radius: 50%;
  &:hover {
    cursor: pointer;
    background: ${props => props.theme.colors.secondary.main};
  }
  transition: background linear 0.1s;
`;
