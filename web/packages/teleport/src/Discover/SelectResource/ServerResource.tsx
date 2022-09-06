import React from 'react';

import { Link, Text } from 'design';

import { ActionButtons, TextBox } from 'teleport/Discover/Shared';

import { PermissionsErrorMessage } from './PermissionsErrorMessage';

export function ServerResource(props: ServerResourceProps) {
  let content = <TeleportVersions />;
  if (props.disabled) {
    content = (
      <PermissionsErrorMessage
        action="add new Servers"
        productName="Server Access"
      />
    );
  }

  return (
    <>
      {content}

      <ActionButtons
        onProceed={() => props.onProceed()}
        disableProceed={props.disabled}
      />
    </>
  );
}

function TeleportVersions() {
  return (
    <TextBox>
      <Text typography="h5">
        Teleport officially supports the following operating systems:
      </Text>
      <ul style={{ paddingLeft: 28 }}>
        <li>Ubuntu 14.04+</li>
        <li>Debian 8+</li>
        <li>RHEL/CentOS 7+</li>
        <li>Amazon Linux 2</li>
        <li>macOS (Intel)</li>
      </ul>
      <Text>
        For a more comprehensive list, visit{' '}
        <Link href="https://goteleport.com/download" target="_blank">
          https://goteleport.com/download
        </Link>
        .
      </Text>
    </TextBox>
  );
}

interface ServerResourceProps {
  disabled: boolean;
  onProceed: () => void;
}
