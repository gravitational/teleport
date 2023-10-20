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

/* MIT License

Copyright (c) 2020 Phosphor Icons

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

import React from 'react';

import { Icon, IconProps } from '../Icon';

/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/

export function Camera({ size = 24, color, ...otherProps }: IconProps) {
  return (
    <Icon
      size={size}
      color={color}
      className="icon icon-camera"
      {...otherProps}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3 5.25C2.17157 5.25 1.5 5.92157 1.5 6.75V17.25C1.5 18.0784 2.17157 18.75 3 18.75H18C18.8284 18.75 19.5 18.0784 19.5 17.25V14.9013L22.8341 17.124C23.0642 17.2775 23.3601 17.2918 23.604 17.1613C23.8478 17.0307 24.0001 16.7766 24.0001 16.5V7.5C24.0001 7.2234 23.8478 6.96926 23.604 6.83875C23.3601 6.70823 23.0642 6.72254 22.8341 6.87596L19.5 9.09867V6.75C19.5 5.92157 18.8284 5.25 18 5.25H3ZM19.5 10.9014V13.0986L22.5001 15.0986V8.90139L19.5 10.9014ZM3 6.75H18V17.25H3V6.75Z"
      />
    </Icon>
  );
}
