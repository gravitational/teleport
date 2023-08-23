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

export function AccessRequestsIcon({ size = 24, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 21 24" size={size} fill={fill}>
      <path d="M7.5 7.875V5.625C7.5 5.00156 6.99844 4.5 6.375 4.5H4.5V0.375C4.5 0.16875 4.33125 0 4.125 0H3.375C3.16875 0 3 0.16875 3 0.375V4.5H1.125C0.501562 4.5 0 5.00156 0 5.625V7.875C0 8.49844 0.501562 9 1.125 9H3V23.625C3 23.8312 3.16875 24 3.375 24H4.125C4.33125 24 4.5 23.8312 4.5 23.625V9H6.375C6.99844 9 7.5 8.49844 7.5 7.875ZM6 7.5H1.5V6H6V7.5ZM13.125 15H11.25V0.375C11.25 0.16875 11.0813 0 10.875 0H10.125C9.91875 0 9.75 0.16875 9.75 0.375V15H7.875C7.25156 15 6.75 15.5016 6.75 16.125V18.375C6.75 18.9984 7.25156 19.5 7.875 19.5H9.75V23.625C9.75 23.8312 9.91875 24 10.125 24H10.875C11.0813 24 11.25 23.8312 11.25 23.625V19.5H13.125C13.7484 19.5 14.25 18.9984 14.25 18.375V16.125C14.25 15.5016 13.7484 15 13.125 15ZM12.75 18H8.25V16.5H12.75V18ZM19.875 7.5H18V0.375C18 0.16875 17.8312 0 17.625 0H16.875C16.6688 0 16.5 0.16875 16.5 0.375V7.5H14.625C14.0016 7.5 13.5 8.00156 13.5 8.625V10.875C13.5 11.4984 14.0016 12 14.625 12H16.5V23.625C16.5 23.8312 16.6688 24 16.875 24H17.625C17.8312 24 18 23.8312 18 23.625V12H19.875C20.4984 12 21 11.4984 21 10.875V8.625C21 8.00156 20.4984 7.5 19.875 7.5ZM19.5 10.5H15V9H19.5V10.5Z" />
    </SVGIcon>
  );
}
