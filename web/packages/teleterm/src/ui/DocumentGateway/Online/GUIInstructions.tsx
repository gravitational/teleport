import React from 'react';

import { Link, Text } from 'design';

import type { Gateway } from 'teleterm/services/tshd/types';

interface GUIInstructionsProps {
  gateway: Gateway;
}

export function GUIInstructions(props: GUIInstructionsProps) {
  return (
    <>
      <Text typography="h4" mt={3} mb={1}>
        Connect with GUI
      </Text>
      <Text
        // Break long usernames rather than putting an ellipsis.
        css={`
          word-break: break-word;
        `}
      >
        Configure the GUI database client to connect to host{' '}
        <code>{props.gateway.localAddress}</code> on port{' '}
        <code>{props.gateway.localPort}</code> as user{' '}
        <code>{props.gateway.targetUser}</code>.
      </Text>
      <Text>
        The connection is made through an authenticated proxy so no extra
        credentials are necessary. See{' '}
        <Link
          href="https://goteleport.com/docs/database-access/guides/gui-clients/"
          target="_blank"
        >
          the documentation
        </Link>{' '}
        for more details.
      </Text>
    </>
  );
}
