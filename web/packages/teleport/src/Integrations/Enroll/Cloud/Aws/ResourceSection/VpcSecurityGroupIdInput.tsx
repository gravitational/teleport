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
import styled from 'styled-components';

import { Box, ButtonIcon, ButtonSecondary, Flex } from 'design';
import { buttonSizes } from 'design/ButtonIcon';
import { Cross } from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';
import { LabelContent } from 'design/LabelInput/LabelInput';
import { ButtonWithAddIcon } from 'shared/components/ButtonWithAddIcon';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import { Regions as AwsRegion } from 'teleport/services/integrations';

import { RegionSelect } from '../../Shared/RegionSelect';
import { awsRegionOptionGroups, awsRegionOptions } from '../regions';
import { DbSvcDeployment } from '../types';

type Region = DbSvcDeployment['region'];

type Deployment = {
  deployment: DbSvcDeployment;
  index: number;
};

export function VpcSecurityGroupIdInput({
  agents,
  onAgentsChange,
}: {
  agents: DbSvcDeployment[];
  onAgentsChange(agents: DbSvcDeployment[]): void;
}) {
  function updateAgent(
    targetAgentIndex: number,
    patch: Partial<DbSvcDeployment>
  ) {
    onAgentsChange(
      agents.map((agent, agentIndex) => {
        if (agentIndex === targetAgentIndex) {
          return { ...agent, ...patch };
        }
        return agent;
      })
    );
  }

  function removeAgent(index: number) {
    const region = agents[index].region;
    const agentsInRegion = agents.filter(agent => agent.region === region);

    if (agentsInRegion.length === 1) {
      // Leaves an empty row since each region requires at least one vpc id.
      updateAgent(index, { vpcId: '', securityGroupIds: [] });
      return;
    }

    const newList = [...agents];
    newList.splice(index, 1);
    onAgentsChange(newList);
  }

  const deploymentsByRegion = new Map<Region, Deployment[]>();
  agents.forEach((deployment, index) => {
    const group = deploymentsByRegion.get(deployment.region);

    if (group) {
      group.push({ deployment, index });
    } else {
      deploymentsByRegion.set(deployment.region, [{ deployment, index }]);
    }
  });

  return (
    <>
      {[...deploymentsByRegion].map(([region, deployments]) => (
        <Deployments key={region || 'unselected-region'}>
          <RegionField>
            <LabelContent required>Region</LabelContent>
            <RegionSelect
              ariaLabel="Region"
              required
              rule={requiredField<Option<AwsRegion>>('Region is required')}
              options={awsRegionOptionGroups}
              isOptionDisabled={option =>
                agents.some(
                  agent =>
                    agent.region !== region && agent.region === option.value
                )
              }
              value={
                awsRegionOptions.find(option => option.value === region) ?? null
              }
              onChange={option =>
                onAgentsChange(
                  agents.map(agent =>
                    agent.region === region
                      ? { ...agent, region: option?.value ?? '' }
                      : agent
                  )
                )
              }
            />
          </RegionField>
          <Box ml={4}>
            <Table>
              <colgroup>
                <col />
                <col />
                <col />
              </colgroup>
              <thead>
                <tr>
                  <th scope="col">
                    <LabelContent required>VPC ID</LabelContent>
                  </th>
                  <th scope="col">
                    <LabelContent required>Security group IDs</LabelContent>
                  </th>
                </tr>
              </thead>
              <tbody>
                {deployments.map(({ deployment, index }) => (
                  <tr key={index}>
                    <td>
                      <FieldInput
                        required
                        rule={requiredField('VPC ID is required')}
                        placeholder="vpc-0123456789abcdef0"
                        value={deployment.vpcId}
                        mb={0}
                        onChange={e =>
                          updateAgent(index, {
                            vpcId: e.target.value,
                          })
                        }
                      />
                    </td>
                    <td>
                      <SecurityGroupIdsInput
                        value={deployment.securityGroupIds}
                        onChange={securityGroupIds =>
                          updateAgent(index, { securityGroupIds })
                        }
                      />
                    </td>
                    <td>
                      <Flex
                        alignItems="center"
                        height={inputGeometry.medium.height}
                      >
                        <ButtonIcon
                          size={0}
                          title="Remove database service"
                          onClick={() => removeAgent(index)}
                        >
                          <Cross color="text.muted" size="small" />
                        </ButtonIcon>
                      </Flex>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <ButtonWithAddIcon
              Button={ButtonSecondary}
              label="Add Another VPC"
              onClick={e => {
                e.preventDefault();
                onAgentsChange([
                  ...agents,
                  { region, vpcId: '', securityGroupIds: [] },
                ]);
              }}
              size="small"
              pr={3}
              compact={false}
              $inputAlignment
            />
          </Box>
        </Deployments>
      ))}
      <ButtonWithAddIcon
        Button={ButtonSecondary}
        label="Add Another Region"
        disabled={agents.some(agent => agent.region === '')}
        onClick={e => {
          e.preventDefault();
          onAgentsChange([
            ...agents,
            { region: '', vpcId: '', securityGroupIds: [] },
          ]);
        }}
        size="small"
        pr={3}
        compact={false}
        $inputAlignment
      />
    </>
  );
}

const SecurityGroupIdsInput = ({
  value,
  onChange,
}: {
  value: string[];
  onChange(value: string[]): void;
}) => {
  const [inputValue, setInputValue] = useState('');

  return (
    <FieldSelectCreatable<Option, true>
      ariaLabel="Security group IDs"
      required
      rule={requiredField<Option>('At least one security group ID is required')}
      placeholder="sg-0123456789abcdef0"
      isMulti
      isSearchable
      inputValue={inputValue}
      onInputChange={setInputValue}
      menuIsOpen={inputValue.trim().length > 0}
      value={value.map(id => ({ label: id, value: id }))}
      onChange={options => {
        onChange(options.map(option => option.value));
        setInputValue('');
      }}
      createOptionPosition="last"
      components={{
        DropdownIndicator: () => null,
        IndicatorSeparator: () => null,
      }}
      formatCreateLabel={input => `Press Enter to add "${input}"`}
      mb={0}
    />
  );
};

const Table = styled.table`
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;

  th {
    padding: 0 0 ${p => p.theme.space[1]}px 0;
  }

  col:nth-child(1) {
    width: 42%;
  }

  col:nth-child(3) {
    width: ${buttonSizes[0].width};
  }

  td {
    padding: 0;
    vertical-align: top;
    padding-bottom: ${p => p.theme.space[2]}px;

    &:nth-child(1),
    &:nth-child(2) {
      padding-right: ${p => p.theme.space[2]}px;
    }
  }
`;

const Deployments = styled.div`
  margin-bottom: ${p => p.theme.space[4]}px;
`;

const RegionField = styled.div`
  width: 400px;
  margin-bottom: ${p => p.theme.space[2]}px;
`;
