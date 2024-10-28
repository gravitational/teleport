import { forwardRef } from 'react';
import { Link } from 'react-router-dom';
import { Box, ButtonPrimary, ButtonText } from 'design';

import {
  RequestCheckoutWithSlider as SharedRequestCheckout,
  RequestCheckoutProps as SharedRequestCheckoutProps,
  ResourceMap,
  RequestableResourceKind,
} from 'shared/components/AccessRequests/NewRequest';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';

import { useRequestCheckout } from './useRequestCheckout';

import type { TransitionStatus } from 'react-transition-group';

export const RequestCheckout = forwardRef<
  HTMLDivElement,
  Pick<
    SharedRequestCheckoutProps,
    | 'onClose'
    | 'toggleResource'
    | 'appsGrantedByUserGroup'
    | 'userGroupFetchAttempt'
    | 'reset'
    | 'isResourceRequest'
    | 'bulkToggleKubeResources'
  > & {
    selectedResource: RequestableResourceKind;
    addedResources: ResourceMap;
    transitionState: TransitionStatus;
  }
>((props, ref) => {
  const { selectedResource, addedResources, reset } = props;
  const ctx = useTeleportE();
  const state = useRequestCheckout({
    ctx,
    selectedResource,
    addedResources,
    reset,
  });

  return (
    <SharedRequestCheckout
      ref={ref}
      transitionState={props.transitionState}
      {...state}
      {...props}
      SuccessComponent={SuccessActionComponent}
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
