/**
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

import { components } from 'react-select';

import { Box, ButtonIcon, Flex, LabelInput, Link } from 'design';
import { NewTab } from 'design/Icon';
import Select from 'shared/components/Select';

import { Regions, Vpc } from 'teleport/services/integrations';

export type VpcOption = { value: Vpc; label: string; link: string };

export function VpcSelector({
  vpcs,
  selectedVpc,
  onSelectedVpc,
  selectedRegion,
}: {
  vpcs: Vpc[];
  selectedVpc: VpcOption;
  onSelectedVpc(o: VpcOption): void;
  selectedRegion: Regions;
}) {
  const options: VpcOption[] = vpcs?.map(vpc => {
    return {
      value: vpc,
      label: `${vpc.id} ${vpc.name && `(${vpc.name})`}`,
      link: `https://${selectedRegion}.console.aws.amazon.com/vpcconsole/home?region=${selectedRegion}#VpcDetails:VpcId=${vpc.id}`,
    };
  });

  return (
    // TODO(lisa): negative margin was required since the
    // AwsRegionSelector added too much bottom margin.
    // Refactor AwsRegionSelector so margins can be controlled
    // outside of the component (or use flex columns with gap prop)
    <Box width="380px" mb={6} mt={-4}>
      <LabelInput>
        VPC ID
        <Box mt={1} mb={6}>
          <Select
            isSearchable
            value={selectedVpc}
            onChange={onSelectedVpc}
            options={options}
            placeholder="Select a VPC ID"
            components={{ Option }}
            autoFocus
          />
        </Box>
      </LabelInput>
    </Box>
  );
}

const Option = props => {
  const { value, link } = props.data;
  const { id, name } = value;
  return (
    <components.Option {...props}>
      <Flex justifyContent="space-between" alignItems="center">
        <div>
          {id} {name && <Box color="text.muted">{name}</Box>}
        </div>
        <ButtonIcon as={Link} target="_blank" href={link}>
          <NewTab />
        </ButtonIcon>
      </Flex>
    </components.Option>
  );
};
