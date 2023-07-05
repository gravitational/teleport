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

export function SearchIcon({ size = 24, fill = 'white' }: SVGIconProps) {
  return (
    <svg
      data-testid="svg"
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      fill={fill}
      viewBox="0 0 24 24"
    >
      <path d="M10 9A1 1 0 1 0 10 11 1 1 0 1 0 10 9zM7 9A1 1 0 1 0 7 11 1 1 0 1 0 7 9zM13 9A1 1 0 1 0 13 11 1 1 0 1 0 13 9zM22 20L20 22 16 18 16 16 18 16z" />
      <path d="M10,18c-4.4,0-8-3.6-8-8c0-4.4,3.6-8,8-8c4.4,0,8,3.6,8,8C18,14.4,14.4,18,10,18z M10,4c-3.3,0-6,2.7-6,6c0,3.3,2.7,6,6,6 c3.3,0,6-2.7,6-6C16,6.7,13.3,4,10,4z" />
      <path
        d="M15.7 14.5H16.7V18H15.7z"
        transform="rotate(-45.001 16.25 16.249)"
      />
    </svg>
  );
}
