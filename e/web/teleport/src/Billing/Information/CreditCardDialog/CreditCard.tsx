import React from 'react';
import styled from 'styled-components';
import { LabelInput, Flex } from 'design';
import { CardElement } from '@stripe/react-stripe-js';

export default function CreditCard(props: Props) {
  const { mb, width } = props;
  return (
    <Flex mb={mb} width={width} flexDirection="column">
      <LabelInput>Credit Card</LabelInput>
      <StyledCard {...props}>
        <CardElement
          options={{
            style: {
              base: {
                backgroundColor: 'white',
                fontSize: '16px',
                lineHeight: '16px',
                color: '#424770',
                '::placeholder': {
                  color: '#aab7c4',
                },
              },
              invalid: {
                color: '#9e2146',
              },
            },
          }}
        />
      </StyledCard>
    </Flex>
  );
}

type Props = {
  [index: string]: string;
};

const StyledCard = styled(Flex)`
  .StripeElement {
    background: white;
    border: 1px solid #bdcad0;
    border-radius: 4px;
    box-sizing: border-box;
    padding: 4px 16px;
    color: #607d8b;
    display: block;
    font-size: 16px;
    height: 42px;
    position: relative;
    width: 100%;
  }

  .StripeElement .__PrivateStripeElement {
    top: 8px !important;
  }

  .StripeElement--invalid {
    border-color: #f50057;
  }
  .StripeElement--focus {
    border-color: #0091ea;
    box-shadow: inset 0 1px 4px rgba(0, 0, 0, 0.24);
  }
  .StripeElement.PaymentRequestButton {
    padding: 0;
  }
`;
