import { TableRole } from 'e-teleport/AccessListManagement/ViewEditAccessList/DeleteAccessList/types';
import { Role } from 'teleport/services/resources';

export type AccessRoleEditor = {
  onUpdateAccess(accessRoles: Role[]): Promise<TableRole[]>;
  onClose(): void;
};
