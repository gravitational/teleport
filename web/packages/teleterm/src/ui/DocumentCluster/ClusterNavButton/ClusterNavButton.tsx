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
import { space, width } from 'design/system';

import {
  useClusterContext,
  NavLocation,
} from 'teleterm/ui/DocumentCluster/clusterContext';

export default function NavButton(props: NavButtonProps) {
  const { title, ...rest } = props;
  const clusterCtx = useClusterContext();
  const active = clusterCtx.isLocationActive(props.to);

  function handleNavClick() {
    clusterCtx.changeLocation(props.to);
  }

  return (
    <StyledNavButton mr={6} active={active} onClick={handleNavClick} {...rest}>
      {title}
    </StyledNavButton>
  );
}

export type NavButtonProps = {
  title: string;
  to: NavLocation;
  [key: string]: any;
};

const StyledNavButton = styled.button(props => {
  return {
    color: props.active
      ? props.theme.colors.text.main
      : props.theme.colors.text.slightlyMuted,
    cursor: 'pointer',
    fontFamily: 'inherit',
    display: 'inline-flex',
    fontSize: '14px',
    position: 'relative',
    padding: '0',
    marginRight: '24px',
    textDecoration: 'none',
    fontWeight: props.active ? 700 : 400,
    outline: 'inherit',
    border: 'none',
    backgroundColor: 'inherit',
    flexShrink: '0',
    borderRadius: '4px',

    '&:hover, &:focus': {
      background: props.theme.colors.spotBackground[0],
    },
    ...space(props),
    ...width(props),
  };
});
