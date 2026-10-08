/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';

import { GitServerAuthorizeView } from './GitServerAuthorize';

export default {
  title: 'Teleport/GitServers/Authorize',
};

export const Ready = () => {
  const [processing, setProcessing] = useState(false);
  return (
    <GitServerAuthorizeView
      organization="my-org"
      processing={processing}
      error=""
      onAuthorize={() => setProcessing(true)}
    />
  );
};

export const Processing = () => (
  <GitServerAuthorizeView
    organization="my-org"
    processing={true}
    error=""
    onAuthorize={() => {}}
  />
);

export const Failed = () => (
  <GitServerAuthorizeView
    organization="my-org"
    processing={false}
    error='integration "my-github" uses the legacy OAuth callback URL, contact your Teleport administrator to migrate it'
    onAuthorize={() => {}}
  />
);

export const NotFound = () => (
  <GitServerAuthorizeView
    processing={false}
    error='git server "my-git-server" not found'
    onAuthorize={() => {}}
  />
);
