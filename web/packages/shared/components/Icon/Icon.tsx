/*
Copyright 2023 Gravitational, Inc.

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

import styled from 'styled-components';

import { IconName } from "./types";

import * as icons from "./icons";

export interface IconProps {
  name: IconName;
  size?: "xxs" | "xs" | "sm" | "md" | "lg" | "xl";
  className?: string;
}

export const Icon = ({ name, size = "md", className }: IconProps) => {
  // eslint-disable-next-line import/namespace
  const IconSVG = icons[name];

  return (
    <IconWrapper>
      {IconSVG ?
        <IconSVG className={`icon ${size} ${className}`}/> :
        <span className={`icon ${size} ${className}`}/>}
    </IconWrapper>
  );
};

const IconWrapper = styled.span`
  .icon {
    display: block;
    width: var(--size);
    height: var(--size);
  
    &.xxs {
      --size: 10px;
    }
  
    &.xs {
      --size: 12px;
    }
  
    &.sm {
      --size: 16px;
    }
  
    &.md {
      --size: 24px;
    }
  
    &.lg {
      --size: 32px;
    }
  
    &.xl {
      --size: 40px;
    }
  }
`;
