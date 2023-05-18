import { Text } from 'design';
import React from 'react';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { CancelTextProps } from 'e-teleport/Billing/types';

export const CancelText = ({
  productName,
  stripeSubscriptionCancelAt,
}: CancelTextProps) => (
  <Text>
    <b>Your Teleport {productName} account has been canceled.</b>&nbsp; Your
    account will close on&nbsp;
    {displayUnixShortDate(stripeSubscriptionCancelAt)}, at which time you will
    be issued a final invoice.
  </Text>
);
