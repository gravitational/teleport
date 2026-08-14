import { AwsRole } from 'shared/services/apps';

import cfg from 'teleport/config';
import { App } from 'teleport/services/apps';

export type AWSLoginChoice = {
  id: string;
  label: string;
  requiresRequest: boolean;
  launchUrl: string;
};

export const AWSRoleToLoginChoice = (app: App) => (role: AwsRole) => {
  return {
    id: role.arn,
    label: `${role.accountId}: ${role.display}${role.display !== role.name ? ` (${role.name})` : ''}`,
    requiresRequest: role.requiresRequest,
    launchUrl: cfg.getAppLauncherRoute({
      fqdn: app.fqdn,
      clusterId: app.clusterId,
      publicAddr: app.publicAddr,
      arn: role.arn,
    }),
  } satisfies AWSLoginChoice;
};
