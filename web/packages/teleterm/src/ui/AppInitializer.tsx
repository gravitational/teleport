import React, { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';
import { Indicator } from 'design';

import { useAppContext } from './appContextProvider';
import { initUi } from './initUi';
import ModalsHost from './ModalsHost';
import { LayoutManager } from './LayoutManager';

export const AppInitializer = () => {
  const ctx = useAppContext();
  const [isUiReady, setIsUiReady] = useState(false);

  const initializeApp = useCallback(async () => {
    try {
      await ctx.init();
      await initUi(ctx);
      setIsUiReady(true);
    } catch (error) {
      setIsUiReady(true);
      ctx.notificationsService.notifyError(error?.message);
    }
  }, [ctx]);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  return (
    <>
      <LayoutManager />
      {!isUiReady && (
        <Centered>
          <Indicator delay="short" />
        </Centered>
      )}
      <ModalsHost />
    </>
  );
};

const Centered = styled.div`
  display: flex;
  position: absolute;
  width: 100%;
  height: 100%;
  justify-content: center;
  align-items: center;
  z-index: 2; // renders the overlay above ConnectionsIconStatusIndicator
  background: ${props => props.theme.colors.primary.darker};
`;
