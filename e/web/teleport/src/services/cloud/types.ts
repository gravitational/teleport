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
