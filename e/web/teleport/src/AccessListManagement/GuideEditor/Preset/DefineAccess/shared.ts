export function getResourceRbacLink(resourceField: 'awsIc') {
  switch (resourceField) {
    case 'awsIc':
      return 'https://goteleport.com/docs/identity-governance/integrations/aws-iam-identity-center/guide/#configure-access-to-the-aws-account-and-permission-set';

    default:
      resourceField satisfies never;
  }
}

export function getResourceKindName(resource: 'awsIc') {
  switch (resource) {
    case 'awsIc':
      return {
        resourceKind: 'AWS Identity Center',
        byline: 'AWS accounts and permission sets',
      };

    default:
      resource satisfies never;
  }
}
