import { useState } from 'react';
import { Link as InternalLink } from 'react-router';

import { ButtonText, Text } from 'design';
import { Warning } from 'design/Alert';

import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'teleport/config';

/**
 * Used as a banner to notify user of RBAC constraints when defining access.
 */
export function PreviewingResourceAlert() {
  const teleCtx = useTeleportE();

  const [showWarning, setShowWarning] = useState(true);

  if (showWarning) {
    return (
      <Warning
        linkColor="buttons.link.default"
        dismissible={true}
        onDismiss={() => setShowWarning(false)}
        mt={2}
      >
        <Text>Previewing Resources</Text>
        <Text fontWeight={300}>
          After defining access for a resource type, the list displays the
          resources members will be able to access. Note that this preview is
          limited by your{' '}
          <InternalLink
            target="_blank"
            to={`${cfg.routes.users}?user=${encodeURIComponent(teleCtx.storeUser.getUsername())}`}
          >
            own role permissions
          </InternalLink>
          ; the user may be granted access to additional resources that are not
          visible to you.
        </Text>
      </Warning>
    );
  }

  return (
    <ButtonText onClick={() => setShowWarning(true)} mb={2} padding="5px">
      Show Warning
    </ButtonText>
  );
}
