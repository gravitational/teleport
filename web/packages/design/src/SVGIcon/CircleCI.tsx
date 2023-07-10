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

export function CircleCIIcon({ size = 104, fill }: SVGIconProps) {
  return (
    <svg
      viewBox="0 0 103.8 105.2"
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      fill={fill}
    >
      <path d="M38.6 52.6c0-6.9 5.6-12.5 12.5-12.5s12.5 5.6 12.5 12.5S58 65.1 51.1 65.1c-6.9.1-12.5-5.6-12.5-12.5ZM51.1 0C26.5 0 5.9 16.8.1 39.6c0 .2-.1.3-.1.5 0 1.4 1.1 2.5 2.5 2.5h21.2c1 0 1.9-.6 2.3-1.5C30.4 31.6 39.9 25 51.1 25c15.2 0 27.6 12.4 27.6 27.6 0 15.2-12.4 27.6-27.6 27.6-11.1 0-20.7-6.6-25.1-16.1-.4-.9-1.3-1.5-2.3-1.5H2.5c-1.4 0-2.5 1.1-2.5 2.5 0 .2 0 .3.1.5 5.8 22.8 26.4 39.6 51 39.6 29.1 0 52.7-23.6 52.7-52.7 0-29-23.6-52.5-52.7-52.5Z" />
    </svg>
  );
}
