import { useState } from 'react';
import { Link as InternalLink } from 'react-router-dom';

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
          When defining access for a resource, we will attempt to list a preview
          of what resources the member may see in their own account. Note that
          this preview is dependent on what your{' '}
          <InternalLink
            target="_blank"
            to={`${cfg.routes.users}?user=${encodeURIComponent(teleCtx.storeUser.getUsername())}`}
          >
            assigned roles
          </InternalLink>{' '}
          allow you to see. <br />
          <Text>
            Access to more resources than what are seen here may be granted.
          </Text>
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
