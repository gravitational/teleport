export const noEditAcessMsg =
  'You do not have permission to edit this Access List';

const managedByOktaMsg =
  'this Access List is managed by Okta and is read-only in Teleport';

export function oktaReadOnlyMsg({
  accessKind,
  userKind,
}: {
  userKind: 'member' | 'owner';
  accessKind: 'eligibility' | 'edit-user' | 'granted-perms';
}) {
  if (accessKind === 'edit-user') {
    return `Editing ${userKind}s is disabled; ${managedByOktaMsg}`;
  }

  if (accessKind === 'eligibility') {
    return `Editing ${userKind} eligibility is disabled; ${managedByOktaMsg}`;
  }

  if (accessKind === 'granted-perms') {
    return `Editing granted permissions is disabled; ${managedByOktaMsg}`;
  }
}
