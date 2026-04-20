import {
  GetAccountUpgradeWindowStartHourResponse as CloudGetAccountUpgradeWindowStartHourResponse,
  UpdateAccountUpgradeWindowStartHourRequest as CloudUpdateAccountUpgradeWindowStartHourRequest,
} from './v1/tenants_pb';

export type GetAccountUpgradeWindowStartHourResponse =
  CloudGetAccountUpgradeWindowStartHourResponse;
export type UpdateAccountUpgradeWindowStartHourRequest =
  CloudUpdateAccountUpgradeWindowStartHourRequest;

export type NonBillableSummaryInformation = {
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
