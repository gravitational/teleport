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

export function ExternalLinkIcon({ size = 22, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 32 32" size={size} fill={fill}>
      <g clipPath="url(#clip)">
        <path d="M31.2 0H23.2C22.758 0 22.4 0.358 22.4 0.8C22.4 1.242 22.758 1.6 23.2 1.6H29.333L11.435 19.434C11.121 19.746 11.121 20.252 11.433 20.565C11.59 20.722 11.795 20.8 11.999 20.8C12.203 20.8 12.407 20.722 12.564 20.566L30.399 2.795V8.8C30.399 9.242 30.757 9.6 31.199 9.6C31.641 9.6 31.999 9.242 31.999 8.8V0.8C31.999 0.358 31.641 0 31.199 0L31.2 0Z" />
        <path d="M26.4 32.0002H2.4C1.077 32.0002 0 30.9232 0 29.6002V5.6002C0 4.2772 1.077 3.2002 2.4 3.2002H18.4C18.842 3.2002 19.2 3.5582 19.2 4.0002C19.2 4.4422 18.842 4.8002 18.4 4.8002H2.4C1.958 4.8002 1.6 5.1582 1.6 5.6002V29.6002C1.6 30.0422 1.958 30.4002 2.4 30.4002H26.4C26.842 30.4002 27.2 30.0422 27.2 29.6002V13.6002C27.2 13.1582 27.558 12.8002 28 12.8002C28.442 12.8002 28.8 13.1582 28.8 13.6002V29.6002C28.8 30.9232 27.723 32.0002 26.4 32.0002V32.0002Z" />
      </g>
      <defs>
        <clipPath id="clip">
          <rect width="32" height="32" fill="white" />
        </clipPath>
      </defs>
    </SVGIcon>
  );
}
