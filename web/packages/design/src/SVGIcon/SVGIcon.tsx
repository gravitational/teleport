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

import React from 'react';
import { useTheme } from 'styled-components';

export function SVGIcon({
  children,
  viewBox = '0 0 20 20',
  size = 20,
  height,
  width,
  fill,
  ...svgProps
}: Props & React.SVGProps<SVGSVGElement>) {
  const theme = useTheme();

  return (
    <svg
      data-testid="svg"
      viewBox={viewBox}
      xmlns="http://www.w3.org/2000/svg"
      width={width || size}
      height={height || size}
      fill={fill || theme.colors.text.main}
      {...svgProps}
    >
      {children}
    </svg>
  );
}

interface Props {
  children: React.SVGProps<SVGPathElement> | React.SVGProps<SVGPathElement>[];
  fill?: string;
  size?: number;
  height?: number;
  width?: number;
  viewBox?: string;
}
