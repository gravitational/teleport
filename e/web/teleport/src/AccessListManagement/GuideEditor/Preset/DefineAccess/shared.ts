import { DefinableResourceAccessFields } from '../role/listaccess';

export function getResourceRbacLink(
  resourceField: DefinableResourceAccessFields
) {
  switch (resourceField) {
    case 'awsIc':
      return 'https://goteleport.com/docs/identity-governance/integrations/aws-iam-identity-center/guide/#configure-access-to-the-aws-account-and-permission-set';
    case 'app_labels':
      return 'https://goteleport.com/docs/enroll-resources/application-access/configuration/controls/';
    case 'db_labels':
      return 'https://goteleport.com/docs/enroll-resources/database-access/rbac/';
    case 'kubernetes_labels':
      return 'https://goteleport.com/docs/enroll-resources/kubernetes-access/controls/';
    case 'node_labels':
      return 'https://goteleport.com/docs/enroll-resources/server-access/rbac/';
    case 'windows_desktop_labels':
      return 'https://goteleport.com/docs/enroll-resources/desktop-access/rbac/';
    case 'github_permissions':
      return 'https://goteleport.com/docs/enroll-resources/application-access/cloud-apis/github-integration/#step-34-configure-access';
    default:
      resourceField satisfies never;
  }
}

export function getResourceKindName(resource: DefinableResourceAccessFields) {
  switch (resource) {
    case 'awsIc':
      return {
        resourceKind: 'AWS Identity Center',
        byline: 'AWS accounts and permission sets',
      };
    case 'app_labels':
      return {
        resourceKind: 'application',
        byline: 'applications',
      };
    case 'db_labels':
      return {
        resourceKind: 'database',
        byline: 'databases',
      };
    case 'kubernetes_labels':
      return {
        resourceKind: 'Kubernetes cluster',
        byline: 'Kubernetes clusters',
      };
    case 'node_labels':
      return {
        resourceKind: 'server',
        byline: 'servers',
      };
    case 'windows_desktop_labels':
      return {
        resourceKind: 'Windows desktop',
        byline: 'Windows desktops',
      };
    case 'github_permissions':
      return {
        resourceKind: 'Git server',
        byline: 'Git servers',
      };
    default:
      resource satisfies never;
  }
}
