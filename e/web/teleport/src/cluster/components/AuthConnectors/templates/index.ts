const saml = require('!raw-loader!./saml.yaml');
const github = require('!raw-loader!./github.yaml');
const oidc = require('!raw-loader!./oidc.yaml');

const templates = {
  saml,
  github,
  oidc,
};

export default templates;
