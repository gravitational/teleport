import { Text } from 'design';

import { AwsIcConfigureIdentitySource } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/IdentitySource';
import { AwsIcImportResources } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/ImportResources';
import { AwsIcOidcIntegration } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/OidcIntegration';
import { AwsIcConfigureScim } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/Scim';
import { PluginEnrollSuccess } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/PluginEnrollSuccess';
import type { CloudHostablePlugin } from 'e-teleport/services/plugins';

export const AwsIdentityCenterPlugin: CloudHostablePlugin = {
  type: 'aws-identity-center',
  name: 'AWS IAM Identity Center',
  icon: 'aws',
  url: 'https://goteleport.com/docs/application-access/okta/guide/',
  cloudHostable: true,
  selfHostable: true,
  fullName: 'AWS IAM Identity Center',
  views: () => {
    return [
      { title: 'AWS Integration', component: AwsIcOidcIntegration },
      {
        title: 'Import Resources',
        component: AwsIcImportResources,
      },
      {
        title: 'External Identity Source',
        component: AwsIcConfigureIdentitySource,
      },
      { title: 'SCIM', component: AwsIcConfigureScim },
      { title: 'Finished', component: PluginEnrollSuccess, hide: false },
    ];
  },
  NextSteps: () => {
    return (
      <Text>
        It will take a while before all users, user groups and permission sets
        are synced between Teleport and AWS IAM Identity Center.
      </Text>
    );
  },
};
