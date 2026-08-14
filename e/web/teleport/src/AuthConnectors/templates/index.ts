import { github } from 'teleport/AuthConnectors/templates';

import oidc from './oidc.yaml?raw';
import saml from './saml.yaml?raw';

const templates = {
  saml,
  oidc,
  github,
};

export default templates;
