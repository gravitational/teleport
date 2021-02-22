import { github } from 'teleport/AuthConnectors/templates';

const saml = require('!raw-loader!./saml.yaml');
const oidc = require('!raw-loader!./oidc.yaml');

const templates = {
  saml,
  github,
  oidc,
};

export default templates;
