import React from 'react';
/*eslint import/named : 0 */
import { Stripe, loadStripe, CreatePaymentMethodData } from '@stripe/stripe-js';
import {
  Elements,
  useElements,
  useStripe,
  CardElement,
} from '@stripe/react-stripe-js';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Attempt } from 'shared/hooks/useAttemptNext';
import * as Alerts from 'design/Alert';
import { ButtonSecondary, ButtonPrimary } from 'design';
import CreditCard from './CreditCard';

export type Props = {
  onSave(stripe: Stripe, data: CreatePaymentMethodData): void;
  onClose(): void;
  stripeKey: string;
  attempt: Attempt;
};

export default function Container(props: Props) {
  const stripePromise = React.useMemo(() => loadStripe(props.stripeKey), []);
  return (
    <Elements stripe={stripePromise}>
      <CreditCardDialog {...props} />
    </Elements>
  );
}

export function CreditCardDialog(props: Props) {
  const { onClose, onSave, attempt } = props;
  const stripe = useStripe();
  const elements = useElements();

  function handleOnSave(e) {
    e.preventDefault();
    const cardElement = elements.getElement(CardElement);
    const paymentMethod = {
      type: 'card',
      card: cardElement,
    } as const;

    onSave(stripe, paymentMethod);
  }

  return (
    <Dialog
      open={true}
      disableEscapeKeyDown={false}
      onClose={onClose}
      dialogCss={() => ({
        maxWidth: '480px',
        width: '100%',
      })}
    >
      <DialogHeader>
        <DialogTitle>Add New Credit Card</DialogTitle>
      </DialogHeader>
      <DialogContent mb={6}>
        {attempt.status === 'failed' && (
          <Alerts.Danger> {attempt.statusText}</Alerts.Danger>
        )}
        <CreditCard width="100%" />
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          disabled={attempt.status === 'processing'}
          onClick={handleOnSave}
          mr="3"
        >
          save
        </ButtonPrimary>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={() => onClose()}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
