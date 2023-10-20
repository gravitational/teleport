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

export function LabelIcon({ size = 14, fill = 'white' }: SVGIconProps) {
  return (
    <svg
      viewBox="0 0 64 64"
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      fill={fill}
    >
      <path d="M 50.998047 5.5371094 A 7.36 7.36 0 0 0 50.890625 5.5390625 L 36.919922 6 A 7.33 7.33 0 0 0 31.919922 8.1503906 L 7.3203125 32.730469 A 7.35 7.35 0 0 0 7.3203125 43.130859 L 20.869141 56.679688 A 7.35 7.35 0 0 0 31.269531 56.679688 L 55.890625 32.060547 A 7.33 7.33 0 0 0 58.039062 27.060547 L 58.460938 13.060547 A 7.38 7.38 0 0 0 56.310547 7.6894531 A 7.36 7.36 0 0 0 50.998047 5.5371094 z M 51 9.5800781 L 51.099609 9.5800781 A 3.35 3.35 0 0 1 54.470703 13 L 54 27 A 3.34 3.34 0 0 1 53 29.269531 L 28.439453 53.859375 A 3.36 3.36 0 0 1 23.699219 53.859375 L 10.140625 40.300781 A 3.36 3.36 0 0 1 10.140625 35.560547 L 34.769531 10.929688 A 3.34 3.34 0 0 1 37 10 L 51 9.5800781 z M 43.365234 16.466797 A 4.12 4.12 0 0 0 40.519531 17.669922 A 4.11 4.11 0 1 0 46.339844 17.669922 A 4.12 4.12 0 0 0 43.365234 16.466797 z M 43.4375 20.474609 A 0.11 0.11 0 0 1 43.519531 20.509766 A 0.11 0.11 0 0 1 43.509766 20.650391 L 43.519531 20.669922 A 0.12 0.12 0 0 1 43.359375 20.669922 A 0.11 0.11 0 0 1 43.359375 20.509766 A 0.11 0.11 0 0 1 43.4375 20.474609 z" />
    </svg>
  );
}
