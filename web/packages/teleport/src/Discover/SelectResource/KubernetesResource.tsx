import React from 'react';

import { ActionButtons } from 'teleport/Discover/Shared';

import { PermissionsErrorMessage } from './PermissionsErrorMessage';

export function KubernetesResource(props: KubernetesResourceProps) {
  let content;
  if (props.disabled) {
    content = (
      <PermissionsErrorMessage
        action="add new Kubernetes resources"
        productName="Kubernetes Access"
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

interface KubernetesResourceProps {
  disabled: boolean;
  onProceed: () => void;
}
