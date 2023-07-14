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

import type { SVGIconProps } from './common';

export function ServerIcon({ size = 13, fill = 'white' }: SVGIconProps) {
  return (
    <svg
      version="1.1"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill={fill}
    >
      <path d="M 7 1 C 5.895 1 5 1.895 5 3 L 5 21 C 5 22.105 5.895 23 7 23 L 17 23 C 18.105 23 19 22.105 19 21 L 19 3 C 19 1.895 18.105 1 17 1 L 7 1 z M 7 3 L 17 3 L 17 13 L 7 13 L 7 3 z M 9 5 L 9 7 L 15 7 L 15 5 L 9 5 z M 9 9 L 9 11 L 15 11 L 15 9 L 9 9 z M 7 15 L 17 15 L 17 21 L 7 21 L 7 15 z M 12 16.5 C 11.172 16.5 10.5 17.172 10.5 18 C 10.5 18.828 11.172 19.5 12 19.5 C 12.828 19.5 13.5 18.828 13.5 18 C 13.5 17.172 12.828 16.5 12 16.5 z" />
    </svg>
  );
}
