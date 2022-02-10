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

import React from 'react';
import styled from 'styled-components';
import { space, color } from 'design/system';
import * as Icons from 'design/Icon';
import { Flex } from 'design';

const AccordingContext = React.createContext<AccordingContextState>(null);

const Expander: React.FC = props => {
  const [expanded, setExpanded] = React.useState(true);
  const [header, ...children] = React.Children.toArray(props.children);
  const toggle = () => setExpanded(!expanded);

  return (
    <AccordingContext.Provider value={{ expanded, toggle }}>
      {header}
      {children}
    </AccordingContext.Provider>
  );
};

export const ExpanderHeader: React.FC<ExpanderHeaderProps> = props => {
  const { onContextMenu, children, ...styles } = props;
  const ctx = React.useContext(AccordingContext);
  const ArrowIcon = ctx.expanded ? Icons.CarrotDown : Icons.CarrotRight;

  return (
    <StyledHeader {...styles} onContextMenu={onContextMenu}>
      <ArrowIcon
        mr="2"
        color="inherit"
        style={{ fontSize: '12px' }}
        onClick={ctx.toggle}
      />
      <Flex flex="1" overflow="hidden">
        {children}
      </Flex>
    </StyledHeader>
  );
};

export const ExpanderContent = styled(Flex)(props => {
  const ctx = React.useContext(AccordingContext);
  return {
    display: ctx.expanded ? 'block' : 'none',
    color: props.theme.colors.text.secondary,
    flexDirection: 'column',
  };
});

export default Expander;

export const StyledHeader = styled(Flex)(props => {
  const theme = props.theme;
  return {
    width: '100%',
    margin: '0',
    boxSizing: 'border-box',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-start',
    border: 'none',
    cursor: 'pointer',
    outline: 'none',
    textDecoration: 'none',
    lineHeight: '24px',
    fontSize: '12px',
    fontWeight: theme.regular,
    fontFamily: theme.font,
    paddingLeft: theme.space[2] + 'px',
    background: theme.colors.primary.main,
    color: theme.colors.text.primary,
    '&.active': {
      borderLeftColor: theme.colors.accent,
      background: theme.colors.primary.lighter,
      color: theme.colors.primary.contrastText,
      '.marker': {
        background: theme.colors.secondary.light,
      },
    },

    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      background: theme.colors.primary.light,
    },

    height: '36px',
    ...space(props),
    ...color(props),
  };
});

type AccordingContextState = {
  expanded: boolean;
  toggle(): void;
};

type ExpanderHeaderProps = {
  onClick?: () => void;
  onContextMenu?: () => void;
  [key: string]: any;
};
