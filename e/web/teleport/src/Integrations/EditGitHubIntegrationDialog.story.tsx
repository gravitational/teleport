import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { EditGitHubIntegrationDialog } from './EditGitHubIntegrationDialog';

export default {
  title: 'TeleportE/Integrations',
};

export function EditGithubDialog() {
  return (
    <EditGitHubIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.GitHub,
        name: 'some-github-integration-name',
        spec: {
          organization: 'some-organization',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );
}
