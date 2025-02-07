import {
  IntegrationOperations,
  Props,
} from 'teleport/Integrations/Operations/IntegrationOperations';
import { IntegrationKind } from 'teleport/services/integrations';

import { EditGitHubIntegrationDialog } from './EditGitHubIntegrationDialog';
import { EditableIntegrationFieldsE } from './useIntegrations';

export function IntegrationOperationsE(
  props: Props<EditableIntegrationFieldsE>
) {
  const { operation, integration, close, edit } = props;

  if (operation === 'edit' && integration.kind === IntegrationKind.GitHub) {
    return (
      <EditGitHubIntegrationDialog
        integration={integration}
        close={close}
        edit={edit}
      />
    );
  }

  return <IntegrationOperations {...props} />;
}
