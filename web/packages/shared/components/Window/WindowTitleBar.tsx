import React from 'react';

import {
  WindowTitleBarButton,
  WindowTitleBarButtons,
  WindowTitleBarContainer,
} from './WindowComponents';

interface WindowTitleBarProps {
  title: string;
}

export function WindowTitleBar(props: WindowTitleBarProps) {
  return (
    <WindowTitleBarContainer>
      <WindowTitleBarButtons>
        <WindowTitleBarButton style={{ backgroundColor: '#f95e57' }} />
        <WindowTitleBarButton style={{ backgroundColor: '#fbbe2e' }} />
        <WindowTitleBarButton style={{ backgroundColor: '#31c842' }} />
      </WindowTitleBarButtons>
      {props.title}
    </WindowTitleBarContainer>
  );
}
