import { RoleToDelete } from 'e-teleport/AccessListManagement/ViewEditAccessList/DeleteAccessList/types';
import { Role } from 'teleport/services/resources';

export type AccessRoleEditor = {
  onUpdateAccess(accessRoles: Role[]): Promise<RoleToDelete[]>;
  onClose(): void;
  usedTerraform: boolean;
};
