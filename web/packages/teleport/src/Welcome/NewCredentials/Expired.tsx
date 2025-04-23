/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import styled from 'styled-components';

import { H2, P1 } from 'design';

import { OnboardCard } from 'teleport/components/Onboard';

export function Expired({ resetMode = false }) {
  const titleCodeTxt = resetMode ? 'Reset' : 'Invitation';
  const paraCodeTxt = resetMode ? 'reset' : 'invite';

  return (
    <OnboardCard>
      <H2 textAlign="center" mb={3}>
        {titleCodeTxt} Code Expired
      </H2>
      <P1>
        It appears that your {paraCodeTxt} code isn't valid any more. Please
        contact your account administrator and request another {paraCodeTxt}{' '}
        link.
      </P1>
      <P1>
        If you believe this is an issue with the product, please create a
        <GithubLink> GitHub issue</GithubLink>.
      </P1>
    </OnboardCard>
  );
}

const GithubLink = styled.a.attrs({
  href: 'https://github.com/gravitational/teleport/issues/new',
})`
  color: ${props => props.theme.colors.buttons.link.default};

  &:visited {
    color: ${props => props.theme.colors.buttons.link.default};
  }
`;
