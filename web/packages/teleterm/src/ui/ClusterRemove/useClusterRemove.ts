import { useAppContext } from '../appContextProvider';
import useAsync from '../useAsync';
import { useEffect } from 'react';

export function useClusterRemove({ clusterUri, onClose, clusterTitle }: Props) {
  const ctx = useAppContext();
  const [{ status, statusText }, removeCluster] = useAsync(() => {
    return ctx.clustersService.removeCluster(clusterUri);
  });

  useEffect(() => {
    if (status === 'success') {
      onClose();
    }
  }, [status]);

  return {
    status,
    statusText,
    removeCluster,
    onClose,
    clusterUri,
    clusterTitle,
  };
}

export type Props = {
  onClose(): void;
  clusterTitle: string;
  clusterUri: string;
};

export type State = ReturnType<typeof useClusterRemove>;
