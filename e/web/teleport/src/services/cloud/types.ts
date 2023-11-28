// tell eslint to ignore auto-generated proto files
/*eslint import/no-unresolved : 0 */
import cloud from './v1/tenants_pb';

export type AddCardRequest = cloud.AddCardRequest.AsObject;

export type RemoveCardRequest = cloud.RemoveCardRequest.AsObject;

export type UpdateCardRequest = cloud.UpdateCardRequest.AsObject;

export type UpdateEmailRequest = cloud.UpdateEmailRequest.AsObject;

export type UpdatePurchaseOrderRequest =
  cloud.UpdatePurchaseOrderPrefixRequest.AsObject;

export type StripeBillingAddressRequest =
  cloud.StripeBillingAddressRequest.AsObject;

export type SetupIntent = cloud.CreateSetupIntentResponse.AsObject;

export type BillingInformation = cloud.GetBillingInformationResponse.AsObject;

export type BillingSummaryInformation =
  cloud.GetBillingSummaryInformationResponse.AsObject;

export type PaymentsInvoicesInformation =
  cloud.GetPaymentsInvoicesInformationResponse.AsObject;

export type InvoiceSettingsInformation =
  cloud.GetInvoiceSettingsInformationResponse.AsObject;

export type StripeCard = cloud.Card.AsObject;

export type StripeCardList = StripeCard[];

export type StripeInvoiceList = cloud.Invoice.AsObject[];

export type StripeInvoiceBillingAddress = cloud.StripeBillingAddress.AsObject;

export type StripeUsage = cloud.StripeUsage.AsObject;

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
