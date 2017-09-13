const roleTemplate = require('raw-loader!./../../../../../../../teleport/examples/resources/role.yaml');
const authTemplate = require('raw-loader!./../../../../../../../teleport/examples/resources/saml-connector.yaml');
const trustedClusterTemplate = require('raw-loader!./../../../../../../../teleport/examples/resources/trusted_cluster.yaml');

export {
  roleTemplate,
  authTemplate,
  trustedClusterTemplate
}