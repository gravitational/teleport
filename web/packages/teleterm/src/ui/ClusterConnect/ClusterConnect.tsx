import React, { useState } from 'react';
import { ClusterAdd } from './ClusterAdd';
import { ClusterLogin } from './ClusterLogin';
import Dialog from 'design/Dialog';

export function ClusterConnect(props: ClusterConnectProps) {
  const [createdClusterUri, setCreatedClusterUri] = useState<
    string | undefined
  >();
  const clusterUri = props.clusterUri || createdClusterUri;

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '480px',
        width: '100%',
        padding: '20px',
      })}
      disableEscapeKeyDown={false}
      onClose={props.onClose}
      open={true}
    >
      {!clusterUri ? (
        <ClusterAdd onClose={props.onClose} onSuccess={setCreatedClusterUri} />
      ) : (
        <ClusterLogin
          clusterUri={clusterUri}
          onClose={props.onClose}
          onSuccess={() => props.onSuccess(clusterUri)}
        />
      )}
    </Dialog>
  );
}

interface ClusterConnectProps {
  clusterUri?: string;
  onClose(): void;
  onSuccess(clusterUri: string): void;
}
