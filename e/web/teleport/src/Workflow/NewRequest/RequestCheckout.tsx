import { Link } from 'react-router-dom';
import { Box, ButtonPrimary, ButtonText } from 'design';

import {
  RequestCheckout as SharedRequestCheckout,
  RequestCheckoutProps as SharedRequestCheckoutProps,
  ResourceMap,
  ResourceKind,
} from 'shared/components/AccessRequests/NewRequest';

import cfg from 'e-teleport/config';
import useTeleportE from 'e-teleport/useTeleportE';

import { useRequestCheckout } from './useRequestCheckout';

export function RequestCheckout(
  props: Pick<
    SharedRequestCheckoutProps,
    | 'onClose'
    | 'toggleResource'
    | 'appsGrantedByUserGroup'
    | 'userGroupFetchAttempt'
    | 'reset'
    | 'transitionState'
    | 'isResourceRequest'
  > & {
    selectedResource: ResourceKind;
    addedResources: ResourceMap;
  }
) {
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
      {...state}
      {...props}
      SuccessComponent={SuccessActionComponent}
    />
  );
}

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
