/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Box, Flex } from 'design';
import { FieldRadio } from 'design/FieldRadio';

import { TextWeighted, TextWeightedSmall } from '../../Shared';
import { TagsButton, TagsChevron } from '../Tags';
import { DbSvcDeployment } from '../types';
import { VpcSecurityGroupIdInput } from './VpcSecurityGroupIdInput';

export function DatabaseServiceSection({
  check,
  unCheck,
  isChecked,
  agents,
  onAgentsChange,
}: {
  check(): void;
  unCheck(): void;
  isChecked: boolean;
  agents: DbSvcDeployment[];
  onAgentsChange(agents: DbSvcDeployment[]): void;
}) {
  const [expanded, setExpanded] = useState(true);

  return (
    <Box my={3}>
      <TagsButton onClick={() => setExpanded(prev => !prev)}>
        <Flex alignItems="center" gap={1} mb={2}>
          <TagsChevron size="medium" expanded={expanded} />
          <TextWeighted>
            Advanced - <span css="font-style: italic;">Database Service</span>
          </TextWeighted>
        </Flex>
      </TagsButton>
      <TextWeightedSmall ml={4} mt={-2}>
        Teleport needs a Database Service to proxy connections to your databases
      </TextWeightedSmall>
      {expanded && (
        <Box ml={4} mt={2}>
          <FieldRadio
            label={
              <Box>
                <TextWeighted>
                  I already have a Database Service that can reach these
                  databases
                </TextWeighted>
                <TextWeightedSmall>
                  Skip this - Teleport will use your existing Database Service.
                </TextWeightedSmall>
              </Box>
            }
            size="small"
            checked={!isChecked}
            onChange={unCheck}
          />
          <FieldRadio
            label={
              <Box>
                <TextWeighted>
                  Deploy a new Database Service for me
                </TextWeighted>
                <TextWeightedSmall>
                  Adds Terraform configuration that deploys a Database Service
                  for each region and VPC pair configured below.
                  <br />
                  Find your VPC ID: AWS Console &gt; RDS &gt; [database] &gt;
                  Connectivity & security
                </TextWeightedSmall>
              </Box>
            }
            size="small"
            checked={isChecked}
            onChange={check}
            mb={2}
          />
          {isChecked && (
            <Box ml={4} mt={3}>
              <VpcSecurityGroupIdInput
                agents={agents}
                onAgentsChange={onAgentsChange}
              />
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
}
