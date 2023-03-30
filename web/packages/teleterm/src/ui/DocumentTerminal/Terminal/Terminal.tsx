/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useEffect, useRef } from 'react';
import styled, { useTheme } from 'styled-components';
import { Box, Flex } from 'design';
import { debounce } from 'shared/utils/highbar';

import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';

import XTermCtrl from './ctrl';

export default function Terminal(props: Props) {
  const refElement = useRef<HTMLElement>();
  const refCtrl = useRef<XTermCtrl>();
  const fontFamily = useTheme().fonts.mono;

  useEffect(() => {
    const ctrl = new XTermCtrl(props.ptyProcess, {
      el: refElement.current,
      fontFamily,
    });

    ctrl.open();

    ctrl.term.onKey(event => {
      if (event.domEvent.key === 'Enter') {
        handleEnterPress();
      }
    });

    refCtrl.current = ctrl;

    const handleEnterPress = debounce(() => {
      props.onEnterKey && props.onEnterKey();
    }, 100);

    return () => {
      handleEnterPress.cancel();
      ctrl.destroy();
    };
  }, []);

  useEffect(() => {
    if (!refCtrl.current || !props.visible) {
      return;
    }

    refCtrl.current.focus();
    refCtrl.current.requestResize();
  }, [props.visible]);

  return (
    <Flex
      flexDirection="column"
      height="100%"
      width="100%"
      style={{ overflow: 'hidden' }}
    >
      <StyledXterm ref={refElement} />
    </Flex>
  );
}

type Props = {
  ptyProcess: IPtyProcess;
  visible: boolean;
  onEnterKey?(): void;
};

const StyledXterm = styled(Box)`
  height: 100%;
  width: 100%;
  background-color: ${props => props.theme.colors.levels.sunken};
`;
