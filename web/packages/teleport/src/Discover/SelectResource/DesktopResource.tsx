import React from 'react';

import { ActionButtons } from 'teleport/Discover/Shared';

import { PermissionsErrorMessage } from './PermissionsErrorMessage';

export function DesktopResource(props: DesktopResourceProps) {
  let content;
  if (props.disabled) {
    content = (
      <PermissionsErrorMessage
        action="add new Desktops"
        productName="Desktop Access"
      />
    );
  }

  return (
    <>
      {content}

      <ActionButtons
        proceedHref="https://goteleport.com/docs/desktop-access/getting-started/"
        disableProceed={props.disabled}
      />
    </>
  );
}

interface DesktopResourceProps {
  disabled: boolean;
}
