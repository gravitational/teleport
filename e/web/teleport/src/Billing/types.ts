import { Dispatch, SetStateAction } from 'react';

import {
  BillingSummaryInformation,
  PaymentsInvoicesInformation,
  StripeCard,
  StripeCardList,
  StripeInvoiceList,
  StripeUsage,
} from 'e-teleport/services/cloud';

export interface SummaryProps {
  data: BillingSummaryInformation;
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
}

export interface CardsListProps {
  cards: StripeCardList;
  defaultSourceID?: string;
  setPageState: Dispatch<SetStateAction<PaymentsInvoicesInformation>>;
}

export interface InvoiceListProps {
  invoices: StripeInvoiceList;
  productName: string;
}

export interface PaymentAddDialogProps {
  open: boolean;
  reload?: () => void;
  setOpen: Dispatch<SetStateAction<boolean>>;
  title: string;
  description?: string;
  makeDefault?: boolean;
  showDefaultOption?: boolean;
}

export interface PaymentBannerProps {
  productName: string;
  reload: () => void;
  stripeTrialEnd: number;
}

export interface CancelBannerProps {
  productName: string;
  stripeTrialEnd: number;
  stripeMissingPaymentMethod: boolean;
}

export type CycleUsage = {
  name: string;
  total: number;
  max: number;
  percentage: number;
};

export interface ExistingPaymentProps {
  open: boolean;
  setOpen: Dispatch<SetStateAction<boolean>>;
  card: StripeCard;
  reload?: () => void;
}
