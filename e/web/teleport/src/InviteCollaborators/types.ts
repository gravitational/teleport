import { Option } from 'shared/components/Select';

import { User } from 'teleport/services/user';

/**
 * An option type for roles that makes descriptions available, for use with
 * react-select.
 */
type RoleValue = {
  name: string;
  description?: string;
};

/**
 * An Option subtype for roles that makes descriptions available, for use with
 * react-select.
 */
export type RoleOption = Option<RoleValue, string>;

export interface InviteCollaboratorsDialogProps {
  onClose: () => void;
  open: boolean;
}

export type InviteCollaboratorsFormProps = {
  users: Set<string>;
  fetchRoles(search: string): Promise<RoleOption[]>;
  recipientsValue: Option[];
  setRecipientsValue: (recipientsValue: React.SetStateAction<Option[]>) => void;
  selectedRoles: RoleOption[];
  setSelectedRoles: (
    selectedRoles: React.SetStateAction<readonly RoleOption[]>
  ) => void;
  onClose?: (users?: User[]) => void;
  hidden?: boolean;
};

export type EmailPasswordResetDialogProps = {
  username: string;
  onClose: () => void;
};
