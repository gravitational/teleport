import React, { FC, useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';

import { useAppContext } from './appContextProvider';
import { initUi } from './initUi';

export const AppInitializer: FC = props => {
  const ctx = useAppContext();
  const [isUiVisible, setIsUiVisible] = useState(false);

  const initializeApp = useCallback(async () => {
    try {
      await ctx.init();
      setIsUiVisible(true);
      await initUi(ctx);
    } catch (error) {
      setIsUiVisible(true);
      ctx.notificationsService.notifyError(error?.message);
    }
  }, [ctx]);

  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  if (!isUiVisible) {
    return <Centered>Loading</Centered>;
  }

  return <>{props.children}</>;
};

const Centered = styled.div`
  margin: auto;
`;
