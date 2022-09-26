import { github } from 'teleport/AuthConnectors/templates';

import saml from './saml.yaml?raw';
import oidc from './oidc.yaml?raw';

const templates = {
  saml,
  oidc,
  github,
};

export default templates;
