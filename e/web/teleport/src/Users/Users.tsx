import { Users as Component } from 'teleport/Users';

import { InviteCollaboratorsDialog } from 'e-teleport/InviteCollaborators';
import EmailPasswordResetDialog from 'e-teleport/InviteCollaborators/EmailPasswordResetDialog';
import cfg from 'e-teleport/config';

export function Users() {
  return (
    <Component
      InviteCollaborators={cfg.oss.isCloud ? InviteCollaboratorsDialog : null}
      EmailPasswordReset={cfg.oss.isCloud ? EmailPasswordResetDialog : null}
    />
  );
}
