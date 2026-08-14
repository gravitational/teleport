import {
  IntegrationOperations,
  Props,
} from 'teleport/Integrations/Operations/IntegrationOperations';
import { IntegrationKind } from 'teleport/services/integrations';

import { EditGitHubIntegrationDialog } from './EditGitHubIntegrationDialog';
import { GitHubRepoAccessDelete } from './IntegrationEnroll/DeleteDialogs/GitHubRepoAccessDelete';
import { EditableIntegrationFieldsE } from './useIntegrations';

export function IntegrationOperationsE(
  props: Props<EditableIntegrationFieldsE>
) {
  const { operation, integration, close, edit, remove } = props;

  if (operation === 'edit' && integration.kind === IntegrationKind.GitHub) {
    return (
      <EditGitHubIntegrationDialog
        integration={integration}
        close={close}
        edit={edit}
      />
    );
  }

  if (operation === 'delete' && integration.kind === IntegrationKind.GitHub) {
    return (
      <GitHubRepoAccessDelete
        onClose={close}
        onRemove={remove}
        integration={integration}
      />
    );
  }

  return <IntegrationOperations {...props} />;
}
