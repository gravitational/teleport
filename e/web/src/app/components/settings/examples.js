const roleTemplate = require('raw-loader!./../../../../telebase-examples/resources/role.yaml');
const authTemplate = require('raw-loader!./../../../../telebase-examples/resources/saml-connector.yaml');
const trustedClusterTemplate = require('raw-loader!./../../../../telebase-examples/resources/trusted_cluster_enterprise.yaml');

export {
  roleTemplate,
  authTemplate,
  trustedClusterTemplate
}