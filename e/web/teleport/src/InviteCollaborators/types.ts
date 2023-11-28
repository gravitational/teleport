import { User } from 'teleport/services/user';
import { Option } from 'shared/components/Select';

/**
 * An option type for roles that makes descriptions available, for use with
 * react-select.
 */
export type RoleValue = {
  name: string;
  description?: string;
};

/**
 * An Option subtype for roles that makes descriptions available, for use with
 * react-select.
 */
export type RoleOption = Option<RoleValue, string>;

export interface InviteCollaboratorsDialogProps {
  onClose: (users?: User[]) => void;
  open: boolean;
}

export type InviteCollaboratorsFormProps = {
  users: Set<string>;
  roles: RoleOption[];
  recipientsValue: Option[];
  setRecipientsValue: (recipientsValue: React.SetStateAction<Option[]>) => void;
  selectedRoles: RoleOption[];
  setSelectedRoles: (selectedRoles: React.SetStateAction<RoleOption[]>) => void;
  onClose?: (users?: User[]) => void;
  hidden: boolean;
};

export type EmailPasswordResetDialogProps = {
  username: string;
  onClose: () => void;
};
