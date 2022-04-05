import React, { FC, useEffect } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import useAsync from 'teleterm/ui/useAsync';
import styled from 'styled-components';

export const AppInitializer: FC = props => {
  const ctx = useAppContext();
  const [{ status }, init] = useAsync(() => ctx.init());

  useEffect(() => {
    init();
  }, []);

  if (status === 'success' || status === 'error') {
    return <>{props.children}</>;
  }

  return <Centered>Loading</Centered>;
};

const Centered = styled.div`
  margin: auto;
`;
