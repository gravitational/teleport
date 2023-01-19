import React from 'react';

import { WindowTitleBar } from './WindowTitleBar';
import { WindowContainer, WindowContentContainer } from './WindowComponents';

interface WindowProps {
  title: string;
}

export function Window(props: React.PropsWithChildren<WindowProps>) {
  return (
    <WindowContainer>
      <WindowTitleBar title={props.title} />

      <WindowContentContainer>{props.children}</WindowContentContainer>
    </WindowContainer>
  );
}
