/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { styled, useTheme } from 'styled-components';

import Flex from 'design/Flex';
import { Question } from 'design/Icon/Icons/Question';
import { Outline } from 'design/Label/Label';
import { fontWeights } from 'design/theme/typography';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import { ApiBotTrait } from 'teleport/services/bot/types';

import { Panel } from './Panel';

export function Traits(props: { traits: ApiBotTrait[] }) {
  const { traits } = props;

  return (
    <Panel title="Traits" isSubPanel testId="traits-panel">
      <TransposedTable>
        <tbody>
          {traits
            .sort((a, b) => a.name.localeCompare(b.name))
            .map(r => (
              <tr key={r.name}>
                <th scope="row">
                  <Trait traitName={r.name} />
                </th>
                <td>
                  {r.values.length > 0
                    ? r.values.map(v => (
                        <Outline mr="1" key={v}>
                          {v}
                        </Outline>
                      ))
                    : 'no values'}
                </td>
              </tr>
            ))}
        </tbody>
      </TransposedTable>
    </Panel>
  );
}

const traitDescriptions: Record<string, string> = {
  aws_role_arns: 'List of allowed AWS role ARNS',
  azure_identities: 'List of Azure identities',
  db_names: 'List of allowed database names',
  db_roles: 'List of allowed database roles',
  db_users: 'List of allowed database users',
  gcp_service_accounts: 'List of GCP service accounts',
  jwt: 'JWT header used for app access',
  kubernetes_groups: 'List of allowed Kubernetes groups',
  kubernetes_users: 'List of allowed Kubernetes users',
  logins: 'List of allowed logins',
  windows_logins: 'List of allowed Windows logins',
  host_user_gid: 'The group ID to use for auto-host-users',
  host_user_uid: 'The user ID to use for auto-host-users',
  github_orgs: 'List of allowed GitHub organizations for git command proxy',
};

function Trait(props: { traitName: string }) {
  const theme = useTheme();

  const description = traitDescriptions[props.traitName];

  const help = (
    <Question
      size={'small'}
      color={theme.colors.interactive.tonal.neutral[3]}
    />
  );

  return description ? (
    <Flex gap={1}>
      {props.traitName}
      <HoverTooltip placement="top" tipContent={description}>
        {help}
      </HoverTooltip>
    </Flex>
  ) : (
    props.traitName
  );
}

const TransposedTable = styled.table`
  th {
    text-align: start;
    padding-right: 16px;
    width: 1%; // Minimum width to fit content
    color: ${({ theme }) => theme.colors.text.muted};
    font-weight: ${fontWeights.regular};
  }
`;
