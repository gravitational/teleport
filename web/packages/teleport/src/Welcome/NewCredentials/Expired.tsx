/*
Copyright 2019-2022 Gravitational, Inc.

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
import styled from 'styled-components';
import { Card, Text } from 'design';

export function Expired({ resetMode = false }) {
  const titleCodeTxt = resetMode ? 'Reset' : 'Invitation';
  const paraCodeTxt = resetMode ? 'reset' : 'invite';

  return (
    <Card
      width="540px"
      color="text.primaryInverse"
      p={6}
      bg="light"
      mt={6}
      mx="auto"
    >
      <Text typography="h1" textAlign="center" fontSize={8} color="text" mb={3}>
        {titleCodeTxt} Code Expired
      </Text>
      <Text typography="paragraph" mb="2">
        It appears that your {paraCodeTxt} code isn't valid any more. Please
        contact your account administrator and request another {paraCodeTxt}{' '}
        link.
      </Text>
      <Text typography="paragraph">
        If you believe this is an issue with the product, please create a
        <GithubLink> GitHub issue</GithubLink>.
      </Text>
    </Card>
  );
}

const GithubLink = styled.a.attrs({
  href: 'https://github.com/gravitational/teleport/issues/new',
})`
  color: ${props => props.theme.colors.link};
  &:visted {
    color: ${props => props.theme.colors.link};
  }
`;
