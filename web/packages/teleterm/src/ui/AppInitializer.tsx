import React, { ReactNode, useEffect } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';

interface AppInitializerProps {
  children?: ReactNode;
}

export function AppInitializer(props: AppInitializerProps) {
  const ctx = useAppContext();
  useEffect(() => {
    const { rootClusterUri } = ctx.statePersistenceService.getWorkspaces();
    if (rootClusterUri) {
      ctx.workspacesService.setActiveWorkspace(rootClusterUri);
    }
  }, []);

  return <>{props.children}</>;
}
