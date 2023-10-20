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

import { SVGIcon } from './SVGIcon';

import type { SVGIconProps } from './common';

export function LogoutIcon({ size = 18, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 18 17" size={size} fill={fill}>
      <path d="M17.8348 7.97773L12.7723 2.91523C12.5525 2.69551 12.1964 2.69551 11.977 2.91523C11.7577 3.13496 11.7573 3.49109 11.977 3.71047L16.0805 7.8125H6.1875C5.87813 7.8125 5.625 8.06563 5.625 8.375C5.625 8.68437 5.87658 8.9375 6.1875 8.9375H16.0805L11.9777 13.0402C11.758 13.26 11.758 13.6161 11.9777 13.8355C12.1975 14.0548 12.5536 14.0552 12.773 13.8355L17.8355 8.77297C17.9438 8.66328 18 8.51914 18 8.375C18 8.23086 17.9438 8.08672 17.8348 7.97773ZM6.1875 15.125H2.8125C1.88191 15.125 1.125 14.3691 1.125 13.4375V3.3125C1.125 2.38191 1.88191 1.625 2.8125 1.625H6.1875C6.49687 1.625 6.75 1.37328 6.75 1.0625C6.75 0.751719 6.49687 0.5 6.1875 0.5H2.8125C1.26141 0.5 0 1.76141 0 3.3125V13.4375C0 14.9879 1.26141 16.25 2.8125 16.25H6.1875C6.49687 16.25 6.75 15.9969 6.75 15.6875C6.75 15.3781 6.49687 15.125 6.1875 15.125Z" />
    </SVGIcon>
  );
}
