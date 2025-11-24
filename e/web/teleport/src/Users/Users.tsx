import cfg from 'e-teleport/config';
import { InviteCollaboratorsDialog } from 'e-teleport/InviteCollaborators';
import EmailPasswordResetDialog from 'e-teleport/InviteCollaborators/EmailPasswordResetDialog';
import { Users as Component } from 'teleport/Users';

import { UserDetails as UserDetailsE } from './UserDetails';

export function Users() {
  return (
    <Component
      InviteCollaborators={cfg.oss.isCloud ? InviteCollaboratorsDialog : null}
      EmailPasswordReset={cfg.oss.isCloud ? EmailPasswordResetDialog : null}
      UserDetails={UserDetailsE}
    />
  );
}
