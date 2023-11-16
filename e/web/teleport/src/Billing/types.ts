import { Dispatch, SetStateAction } from 'react';

import {
  BillingSummaryInformation,
  InvoiceSettingsInformation,
  PaymentsInvoicesInformation,
  StripeCard,
  StripeCardList,
  StripeInvoiceBillingAddress,
  StripeInvoiceList,
  StripeUsage,
  NonBillableSummaryInformation,
} from 'e-teleport/services/cloud';

export interface SummaryProps {
  data: BillingSummaryInformation;
  nonBillableUsage: NonBillableSummaryInformation;
  reload: () => void;
}

export interface InvoiceSettingsProps {
  data: InvoiceSettingsInformation;
  reload: () => void;
}

export interface AddressProps {
  address?: StripeInvoiceBillingAddress;
  name?: string;
  reload: () => void;
}

export interface PaymentsAndInvoicesProps {
  data: PaymentsInvoicesInformation;
  reload: () => void;
}

export interface ErrorPageProps {
  message: string;
}

export interface CycleProps {
  currentUsage: StripeUsage;
  productName: string;
  stripeMissingPaymentMethod: boolean;
  stripeTrialEnd: number;
  usageUpdatedAt: number;
  nonBillableUsage: NonBillableSummaryInformation;
}

export interface CardsListProps {
  cards: StripeCardList;
  defaultSourceID?: string;
  setPageState: Dispatch<SetStateAction<PaymentsInvoicesInformation>>;
  stripeMissingPaymentMethod: boolean;
}

export interface InvoiceListProps {
  invoices: StripeInvoiceList;
  productName: string;
}

export interface EmailProps {
  email?: string;
  reload: () => void;
}

export interface PurchaseOrderProps {
  po?: string;
  reload: () => void;
}

export interface PaymentAddDialogProps {
  open: boolean;
  reload?: () => void;
  setOpen: Dispatch<SetStateAction<boolean>>;
  title: string;
  stripeMissingPaymentMethod: boolean;
  description?: string;
  makeDefault?: boolean;
  showDefaultOption?: boolean;
}

export interface PaymentBannerProps {
  productName: string;
  reload: () => void;
  stripeTrialEnd: number;
}

export interface StatusBannerProps {
  productName: string;
  stripeCurrentPeriodEnd: number;
  stripeMissingPaymentMethod: boolean;
  stripeSubscriptionStatus: string;
  stripeTrialEnd: number;
  stripeSubscriptionCanceled: boolean;
}

export type CycleUsage = {
  name: string;
  total: number;
  percentageMax: number;
  hardMax: number;
  percentage: number;
  hasFreeTier: boolean;
  info: string;
};

export interface ExistingPaymentProps {
  open: boolean;
  setOpen: Dispatch<SetStateAction<boolean>>;
  card: StripeCard;
  reload?: () => void;
}

export interface CancelDialogProps {
  open: boolean;
  setOpen: Dispatch<SetStateAction<boolean>>;
  productName: string;
  stripeCurrentPeriodEnd: number;
  tenant: string;
}

export interface CancelTextProps {
  productName: string;
  stripeSubscriptionCancelAt: number;
}
