// Prefix github org name with `github` to help avoid
// possible name clashes for resource type "integration"
// and resource type "server" with subKind "git".
export function getIntegrationName(gitHubOrgName: string) {
  return `github-${gitHubOrgName}`;
}
