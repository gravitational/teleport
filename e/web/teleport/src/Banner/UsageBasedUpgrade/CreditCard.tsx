import React from 'react';
import { Flex, LabelInput } from 'design';
import { useTheme } from 'styled-components';
import { CardElement } from '@stripe/react-stripe-js';

import { CreditCardProps } from 'e-teleport/Banner/UsageBasedUpgrade/types';

export const CreditCard = ({ setValid }: CreditCardProps) => {
  const theme = useTheme();
  return (
    <Flex flexDirection="column">
      <LabelInput>Credit Card</LabelInput>
      <CardElement
        options={{
          style: {
            base: {
              backgroundColor: theme.colors.spotBackground[0],
              fontSize: '16px',
              color: theme.colors.text.primary,
              '::placeholder': {
                color: theme.colors.text.placeholder,
              },
            },
            invalid: {
              color: theme.colors.danger,
            },
          },
        }}
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
};
