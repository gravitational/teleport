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

export function AccessListIcon({ size = 18, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 18 18">
      <path
        fillRule="evenodd"
        d="M2.25 7.313a3.375 3.375 0 1 1 5.435 2.673c1.445.614 2.592 1.848 2.985 3.374a.562.562 0 1 1-1.09.28c-.431-1.678-2.1-2.953-3.955-2.953S2.101 11.963 1.67 13.64a.563.563 0 1 1-1.09-.28c.393-1.526 1.54-2.76 2.985-3.374A3.37 3.37 0 0 1 2.25 7.312Zm3.375-2.25a2.25 2.25 0 1 0 0 4.5 2.25 2.25 0 0 0 0-4.5Z"
        clip-rule="evenodd"
      />
      <path
        fill="#000"
        d="M10.125 5.625c0-.31.252-.563.563-.563h6.75a.562.562 0 1 1 0 1.125h-6.75a.562.562 0 0 1-.563-.562ZM10.688 8.438a.562.562 0 1 0 0 1.124h6.75a.562.562 0 1 0 0-1.124h-6.75ZM11.813 12.375c0-.31.251-.563.562-.563h5.063a.562.562 0 1 1 0 1.126h-5.063a.562.562 0 0 1-.563-.563Z"
      />
    </SVGIcon>
  );
}
