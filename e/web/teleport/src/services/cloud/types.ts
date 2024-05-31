// tell eslint to ignore auto-generated proto files
/*eslint import/no-unresolved : 0 */
import cloud from './v1/tenants_pb';

export type AddCardRequest = cloud.AddCardRequest;
export type RemoveCardRequest = cloud.RemoveCardRequest;
export type UpdateCardRequest = cloud.UpdateCardRequest;
export type UpdateEmailRequest = cloud.UpdateEmailRequest;
export type UpdatePurchaseOrderRequest = cloud.UpdatePurchaseOrderPrefixRequest;
export type StripeBillingAddressRequest = cloud.StripeBillingAddressRequest;
export type SetupIntent = cloud.CreateSetupIntentResponse;
export type BillingInformation = cloud.GetBillingInformationResponse;
export type BillingSummaryInformation =
  cloud.GetBillingSummaryInformationResponse;
export type PaymentsInvoicesInformation =
  cloud.GetPaymentsInvoicesInformationResponse;
export type InvoiceSettingsInformation =
  cloud.GetInvoiceSettingsInformationResponse;
export type StripeCard = cloud.Card;
export type StripeCardList = StripeCard[];
export type StripeInvoiceList = cloud.Invoice[];
export type StripeInvoiceBillingAddress = cloud.StripeBillingAddress;
export type StripeUsage = cloud.StripeUsage;
export type GetAccountUpgradeWindowStartHourResponse =
  cloud.GetAccountUpgradeWindowStartHourResponse;
export type UpdateAccountUpgradeWindowStartHourRequest =
  cloud.UpdateAccountUpgradeWindowStartHourRequest;
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
