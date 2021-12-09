import { github } from 'teleport/AuthConnectors/templates';

import saml from 'raw-loader!./saml.yaml';
import oidc from 'raw-loader!./oidc.yaml';

const templates = {
  saml,
  oidc,
  github,
};

export default templates;
