import React from 'react';
import { Flex } from 'design';

import { PaymentElement } from '@stripe/react-stripe-js';

import { CreditCardProps } from 'e-teleport/Banner/UsageBasedUpgrade/types';

export const CreditCard = ({ setValid }: CreditCardProps) => (
  <Flex flexDirection="column">
    <PaymentElement
      onChange={event => {
        if (event.complete) {
          setValid(true);
        } else {
          setValid(false);
        }
      }}
    />
  </Flex>
);
