import React from 'react';

import { Text } from 'design';

import { TextBox } from 'teleport/Discover/Shared';

export function PermissionsErrorMessage(props: PermissionsErrorMessageProps) {
  return (
    <TextBox>
      <Text typography="h5">
        You are not able to {props.action}. There are two possible reasons for
        this:
      </Text>
      <ul style={{ paddingLeft: 28 }}>
        <li>
          Your Teleport Enterprise license does not include {props.productName}.
          Reach out to your Teleport administrator to enable {props.productName}
          .
        </li>
        <li>
          You don’t have sufficient permissions to {props.action}. Reach out to
          your Teleport administrator to request additional permissions.
        </li>
      </ul>
    </TextBox>
  );
}

interface PermissionsErrorMessageProps {
  action: string;
  productName: string;
}
