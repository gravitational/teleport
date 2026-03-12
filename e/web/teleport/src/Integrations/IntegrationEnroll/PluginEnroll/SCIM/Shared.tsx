import { useNavigate } from 'react-router';

import { Alert } from 'design';

import cfg from 'e-teleport/config';

export const SCIMEnrolNoAuthConnectorsAlert = () => {
  const navigate = useNavigate();

  return (
    <Alert
      kind="outline-info"
      mb={0}
      primaryAction={{
        content: 'Add Auth Connector',
        onClick: () => navigate(cfg.routes.ssoNewConnectorList),
      }}
      details="To set up a SCIM integration, you must configure an external auth connector (e.g. Okta, Google Workspace, Azure AD). Local authentication is not supported."
    >
      SCIM Integration Requires External Auth Connector
    </Alert>
  );
};
