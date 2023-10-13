import { AccessListMember } from 'e-teleport/services/accessmanagement';

export function getMembersDeleted(
  originalMembers: AccessListMember[],
  editedMembers: AccessListMember[]
) {
  return originalMembers.filter(
    wasExistedMember => !editedMembers.includes(wasExistedMember)
  );
}
