import { useHistory } from 'react-router';

import { Alert } from 'design';

import cfg from 'e-teleport/config';

export const SCIMEnrolNoAuthConnectorsAlert = () => {
  const history = useHistory();

  return (
    <Alert
      kind="outline-info"
      mb={0}
      primaryAction={{
        content: 'Add Auth Connector',
        onClick: () => history.push(cfg.routes.ssoNewConnectorList),
      }}
      details="To set up a SCIM integration, you must configure an external auth connector (e.g. Okta, Google Workspace, Azure AD). Local authentication is not supported."
    >
      SCIM Integration Requires External Auth Connector
    </Alert>
  );
};
