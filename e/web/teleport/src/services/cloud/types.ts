import {
  AddCardRequest as CloudAddCardRequest,
  Card as CloudCard,
  CreateSetupIntentResponse as CloudCreateSetupIntentResponse,
  GetAccountUpgradeWindowStartHourResponse as CloudGetAccountUpgradeWindowStartHourResponse,
  GetBillingInformationResponse as CloudGetBillingInformationResponse,
  GetBillingSummaryInformationResponse as CloudGetBillingSummaryInformationResponse,
  GetInvoiceSettingsInformationResponse as CloudGetInvoiceSettingsInformationResponse,
  GetPaymentsInvoicesInformationResponse as CloudGetPaymentsInvoicesInformationResponse,
  Invoice as CloudInvoice,
  RemoveCardRequest as CloudRemoveCardRequest,
  StripeBillingAddress as CloudStripeBillingAddress,
  StripeBillingAddressRequest as CloudStripeBillingAddressRequest,
  StripeUsage as CloudStripeUsage,
  UpdateAccountUpgradeWindowStartHourRequest as CloudUpdateAccountUpgradeWindowStartHourRequest,
  UpdateCardRequest as CloudUpdateCardRequest,
  UpdateEmailRequest as CloudUpdateEmailRequest,
  UpdatePurchaseOrderPrefixRequest as CloudUpdatePurchaseOrderPrefixRequest,
} from './v1/tenants_pb';

export type AddCardRequest = CloudAddCardRequest;
export type RemoveCardRequest = CloudRemoveCardRequest;
export type UpdateCardRequest = CloudUpdateCardRequest;
export type UpdateEmailRequest = CloudUpdateEmailRequest;
export type UpdatePurchaseOrderRequest = CloudUpdatePurchaseOrderPrefixRequest;
export type StripeBillingAddressRequest = CloudStripeBillingAddressRequest;
export type SetupIntent = CloudCreateSetupIntentResponse;
export type BillingInformation = CloudGetBillingInformationResponse;
export type BillingSummaryInformation =
  CloudGetBillingSummaryInformationResponse;
export type PaymentsInvoicesInformation =
  CloudGetPaymentsInvoicesInformationResponse;
export type InvoiceSettingsInformation =
  CloudGetInvoiceSettingsInformationResponse;
export type StripeCard = CloudCard;
export type StripeCardList = StripeCard[];
export type StripeInvoiceList = CloudInvoice[];
export type StripeInvoiceBillingAddress = CloudStripeBillingAddress;
export type StripeUsage = CloudStripeUsage;
export type GetAccountUpgradeWindowStartHourResponse =
  CloudGetAccountUpgradeWindowStartHourResponse;
export type UpdateAccountUpgradeWindowStartHourRequest =
  CloudUpdateAccountUpgradeWindowStartHourRequest;

export type NonBillableSummaryInformation = {
  trustedDeviceUsage: {
    devicesUsageLimit: number;
    devicesInUse: number;
  };
  accessRequestUsage: {
    monthlyLimit: number;
    monthlyUsed: number;
  };
};
export type SendTeleportInvite = {
  recipients: Array<string>;
  roles: Array<string>;
};
export type SendTeleportCredentialReset = {
  recipient: string;
};
