import { StripeError } from '@stripe/stripe-js';

import { Dispatch, SetStateAction } from 'react';

import { BillingInformation, StripeCard } from 'e-teleport/services/cloud';

export type NetworkStatus = 'loading' | 'error' | 'success';

export type NetworkState = {
  status?: NetworkStatus;
  error?: Error | StripeError;
};

export interface UsageBasedUpgradeProps {
  billingInfo: BillingInformation;
  reload: () => void;
}

export interface CreditCardProps {
  card?: StripeCard;
  setValid: Dispatch<SetStateAction<boolean>>;
}
