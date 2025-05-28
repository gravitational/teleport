import { OktaIntegrationStepType } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

export type OktaIntegrationStepFormProps =
  | {
      nextStepType?: OktaIntegrationStepType | undefined;
      plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
      previousStepType?: OktaIntegrationStepType | undefined;
      isEditing?: false;
    }
  | {
      nextStepType?: never;
      plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
      previousStepType?: never;
      isEditing: true;
    };
