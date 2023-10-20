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

export function DisplayIcon({ size = 32, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 32 32">
      <path d="M30.662 9.003a98.793 98.793 0 0 0-8.815-.851L27 2.999l-2-2-7.017 7.017c-.656-.011-1.317-.017-1.983-.017l-8-8-2 2 6.069 6.069c-3.779.133-7.386.454-10.731.935C.478 12.369 0 16.089 0 20s.477 7.63 1.338 10.997C5.827 31.642 10.786 32 16 32s10.173-.358 14.662-1.003C31.522 27.631 32 23.911 32 20s-.477-7.63-1.338-10.997zm-3.665 18.328C23.63 27.761 19.911 28 16 28s-7.63-.239-10.997-.669C4.358 25.087 4 22.607 4 20s.358-5.087 1.003-7.331C8.369 12.239 12.089 12 16 12s7.63.239 10.996.669C27.641 14.913 28 17.393 28 20s-.358 5.087-1.003 7.331z" />
    </SVGIcon>
  );
}
