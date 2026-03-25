/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import Flex from 'design/Flex';

import TextEditor from './TextEditor';

export default {
  title: 'Shared/TextEditor',
};

export const Editor = () => {
  const [content, setContent] = useState(EXAMPLE);
  return (
    <Flex height="600px" width="600px" py={3} pr={3} bg="levels.deep">
      <TextEditor
        bg="levels.deep"
        data={[
          {
            content,
            type: 'yaml',
          },
        ]}
        onChange={(value: string) => setContent(value)}
      />
    </Flex>
  );
};

export const ReadOnly = () => {
  const [content, setContent] = useState(EXAMPLE);
  return (
    <Flex height="600px" width="600px" py={3} pr={3} bg="levels.deep">
      <TextEditor
        bg="levels.deep"
        readOnly
        data={[
          {
            content,
            type: 'yaml',
          },
        ]}
        onChange={(value: string) => setContent(value)}
      />
    </Flex>
  );
};

export const WithButtons = () => {
  const [content, setContent] = useState(EXAMPLE);
  return (
    <Flex height="600px" width="600px" py={3} pr={3} bg="levels.deep">
      <TextEditor
        bg="levels.deep"
        data={[
          {
            content,
            type: 'yaml',
          },
        ]}
        onChange={(value: string) => setContent(value)}
        copyButton
        downloadButton
        downloadFileName="content.yaml"
      />
    </Flex>
  );
};

const EXAMPLE = `# example
kind: github
version: v3
metadata:
  name: github
spec:
  client_id: client-id
  client_secret: client-secret
  display: GitHub
  redirect_url: https://tele.example.com:443/v1/webapi/github/callback
  teams_to_roles:
    - organization: octocats
      team: admin
      roles: ["access"]
`;
