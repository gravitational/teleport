import { Text } from 'design';
import React from 'react';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { CancelAtProps } from 'e-teleport/Billing/types';

export const CancelText = ({ stripeSubscriptionCancelAt }: CancelAtProps) => (
  <Text>
    <b>
      Your plan has been canceled effective{' '}
      {displayUnixShortDate(stripeSubscriptionCancelAt)},
    </b>
    at which time you will be issued a final invoice, Teleport access will be
    cut off and your account will be deleted.
  </Text>
);
