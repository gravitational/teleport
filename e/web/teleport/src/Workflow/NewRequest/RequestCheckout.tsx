import { forwardRef } from 'react';
import { Link } from 'react-router';
import type { TransitionStatus } from 'react-transition-group';

import { Box, ButtonPrimary, ButtonText } from 'design';
import {
  ResourceMap,
  RequestCheckoutWithSlider as SharedRequestCheckout,
  RequestCheckoutProps as SharedRequestCheckoutProps,
} from 'shared/components/AccessRequests/NewRequest';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';

import { useRequestCheckout } from './useRequestCheckout';

export const RequestCheckout = forwardRef<
  HTMLDivElement,
  Pick<
    SharedRequestCheckoutProps,
    | 'onClose'
    | 'toggleResource'
    | 'toggleResources'
    | 'appsGrantedByUserGroup'
    | 'userGroupFetchAttempt'
    | 'reset'
    | 'isResourceRequest'
    | 'updateNamespacesForKubeCluster'
    | 'addedResourceConstraints'
    | 'setResourceConstraints'
  > & {
    addedResources: ResourceMap;
    transitionState: TransitionStatus;
  }
>((props, ref) => {
  const {
    isResourceRequest,
    addedResources,
    reset,
    addedResourceConstraints,
    setResourceConstraints,
  } = props;
  const ctx = useTeleportE();
  const state = useRequestCheckout({
    ctx,
    isResourceRequest,
    addedResources,
    addedResourceConstraints,
    setResourceConstraints,
    reset,
  });

  return (
    <SharedRequestCheckout
      ref={ref}
      transitionState={props.transitionState}
      {...state}
      {...props}
      SuccessComponent={SuccessActionComponent}
      reset={state.cancelCheckout}
    />
  );
});

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
        Back to Listings
      </ButtonPrimary>
      <ButtonText
        onClick={() => {
          reset();
          onClose();
        }}
      >
        Make Another Request
      </ButtonText>
    </Box>
  );
}
