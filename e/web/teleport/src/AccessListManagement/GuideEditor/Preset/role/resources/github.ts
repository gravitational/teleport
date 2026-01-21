import {
  GitHubPermission,
  GitHubResourceAccess,
} from 'teleport/services/resources';

/**
 * Access definition for git_server resources.
 *
 * Unlike most resources, git_server access is NOT controlled by
 * labels but by github_permissions (allowed GitHub orgs).
 *
 * Derived from GitHubResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type GitHubIdentities = Required<GitHubResourceAccess> & {
  github_permissions: Required<GitHubPermission>[];
};

export const emptyGitHubIdentities = (): GitHubIdentities => ({
  github_permissions: [],
});

export function getGitHubOrgs(ghPerm?: GitHubPermission[]): string[] {
  if (!ghPerm || !ghPerm.length) {
    return [];
  }
  const orgs: string[] = [];
  ghPerm.forEach(perm => {
    if (perm.orgs) {
      perm.orgs.forEach(org => orgs.push(org));
    }
  });
  return orgs;
}
