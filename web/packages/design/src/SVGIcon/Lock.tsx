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

export function LockIcon({ size = 13, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 10 13" size={size} fill={fill}>
      <path d="M8.1501 4.60002H7.8001V3.55002C7.8001 1.81315 6.38697 0.400024 4.6501 0.400024C2.91322 0.400024 1.5001 1.81315 1.5001 3.55002V4.60002H1.1501C0.571285 4.60002 0.100098 5.07121 0.100098 5.65002V11.25C0.100098 11.8288 0.571285 12.3 1.1501 12.3H8.1501C8.72891 12.3 9.2001 11.8288 9.2001 11.25V5.65002C9.2001 5.07121 8.72891 4.60002 8.1501 4.60002ZM2.2001 3.55002C2.2001 2.19902 3.2991 1.10002 4.6501 1.10002C6.0011 1.10002 7.1001 2.19902 7.1001 3.55002V4.60002H2.2001V3.55002ZM8.5001 11.25C8.5001 11.4434 8.34347 11.6 8.1501 11.6H1.1501C0.956722 11.6 0.800098 11.4434 0.800098 11.25V5.65002C0.800098 5.45665 0.956722 5.30002 1.1501 5.30002H8.1501C8.34347 5.30002 8.5001 5.45665 8.5001 5.65002V11.25Z" />
    </SVGIcon>
  );
}
