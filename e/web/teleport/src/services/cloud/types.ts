import {
  GetAccountUpgradeWindowStartHourResponse as CloudGetAccountUpgradeWindowStartHourResponse,
  GetBillingSummaryInformationResponse as CloudGetBillingSummaryInformationResponse,
  StripeUsage as CloudStripeUsage,
  UpdateAccountUpgradeWindowStartHourRequest as CloudUpdateAccountUpgradeWindowStartHourRequest,
  UpdateEmailRequest as CloudUpdateEmailRequest,
} from './v1/tenants_pb';

export type UpdateEmailRequest = CloudUpdateEmailRequest;
export type BillingSummaryInformation =
  CloudGetBillingSummaryInformationResponse;
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
