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

import { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonText, Flex, H2, Subtitle1, Text } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import * as Icons from 'design/Icon';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import FieldInput from 'shared/components/FieldInput';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation from 'shared/components/Validation';
import {
  requiredAwsAccountId,
  requiredIntegrationName,
  requiredRoleArn,
} from 'shared/components/Validation/rules';

import { LabelsInput, type Label } from 'teleport/components/LabelsInput';
import { Header } from 'teleport/Discover/Shared';
import { Regions } from 'teleport/services/integrations';

import { RegionMultiSelector } from './RegionMultiSelector';
import { TerraformAwsIam } from './terraform';
import { useCloudAws } from './useCloudAws';

type deploymentMethod = 'terraform' | 'cloudformation' | 'manual';

export function CloudAws() {
  const { infoGuideConfig, setInfoGuideConfig } = useInfoGuide();
  const {
    integrationConfig,
    setIntegrationConfig,
    awsConfig,
    setAwsConfig,
    eksConfig,
    setEksConfig,
    ec2Config,
    setEc2Config,
    rdsConfig,
    setRdsConfig,
  } = useCloudAws();
  const [allRegions, setAllRegions] = useState<boolean>(true);
  const [selectedRegions, setSelectedRegions] = useState<Regions[]>([]);
  const [deploymentMethod, setDeploymentMethod] =
    useState<deploymentMethod>('terraform');

  const toggleEks = () => {
    setEksConfig({
      ...eksConfig,
      enabled: !eksConfig.enabled,
    });
  };

  const toggleEc2 = () => {
    setEc2Config({
      ...ec2Config,
      enabled: !ec2Config.enabled,
    });
  };

  const toggleRds = () => {
    setRdsConfig({
      ...rdsConfig,
      enabled: !rdsConfig.enabled,
    });
  };

  useEffect(() => {
    setInfoGuideConfig({
      title: 'Terraform',
      guide: (
        <TerraformAwsIam
          integrationName={integrationConfig.name}
          accountId={awsConfig.accountId}
          regions={awsConfig.regions}
          ec2Enabled={ec2Config.enabled}
          ec2Matchers={ec2Config.matchers}
          rdsEnabled={rdsConfig.enabled}
          rdsMatchers={rdsConfig.matchers}
          eksEnabled={eksConfig.enabled}
          eksMatchers={eksConfig.matchers}
        />
      ),
    });
  }, [
    deploymentMethod,
    integrationConfig.name,
    awsConfig.regions,
    awsConfig.accountId,
    ec2Config,
    rdsConfig,
    eksConfig,
  ]);

  useEffect(() => {
    console.log('awsConfig updated:', awsConfig);
  }, [awsConfig]);

  useEffect(() => {
    console.log('integrationConfig updated:', integrationConfig);
  }, [integrationConfig]);

  return (
    <Validation>
      {({ validator }) => (
        <Box pt={3}>
          <Header>Connect Amazon Web Services</Header>
          <Subtitle1 mb={3}>
            Connect your AWS account to automatically discover and enroll
            resources in your Teleport Cluster.
          </Subtitle1>

          <Container flexDirection="column" p={6}>
            <H2>Integration Details</H2>
            <Text>
              A unique name to identify this AWS integration. This will be used
              to reference the integration in Teleport.
            </Text>

            <FieldInput
              autoFocus={true}
              rule={requiredIntegrationName}
              value={integrationConfig.name}
              required={true}
              label="Integration name"
              placeholder="Integration Name"
              maxWidth={334}
              mt={2}
              onChange={e =>
                setIntegrationConfig({
                  ...integrationConfig,
                  name: e.target.value.trim(),
                })
              }
            />

            <H2>Configure Scope</H2>
            <Text>
              Choose whether to discover resources across your entire AWS
              Organization or a single AWS account.
            </Text>

            <FieldInput
              autoFocus={true}
              rule={requiredAwsAccountId}
              value={awsConfig.accountId}
              required={true}
              label="AWS Account ID"
              placeholder="012345678901"
              maxWidth={334}
              mt={2}
              onChange={e =>
                setAwsConfig({
                  ...awsConfig,
                  accountId: e.target.value.trim(),
                })
              }
            />

            <H2>Resource Types</H2>
            <Text mb={3}>
              Select which AWS resource types to automatically discover and
              enroll.
            </Text>

            <FieldCheckbox
              label="EC2 Instances"
              helperText="Teleport will discover EC2 instances and establish SSH access through the
  Teleport proxy."
              checked={ec2Config.enabled}
              onChange={() =>
                setEc2Config({
                  ...ec2Config,
                  enabled: !ec2Config.enabled,
                })
              }
            />

            <Box ml={4} mb={2}>
              <FilterButton onClick={toggleEc2} size="small">
                <Flex alignItems="center" gap={1}>
                  <FilterChevron size="small" expanded={ec2Config.enabled} />
                  Filter by tag
                  <Icons.Info size="small" ml={1} />
                </Flex>
              </FilterButton>
              {ec2Config.enabled && (
                <Box mt={1} mb={2} width={400}>
                  <LabelsInput
                    adjective="tag"
                    labels={ec2Config.matchers as Label[]}
                    setLabels={(matchers: Label[]) =>
                      setEc2Config({
                        ...ec2Config,
                        matchers: matchers,
                      })
                    }
                  />
                </Box>
              )}
            </Box>

            <FieldCheckbox
              label="RDS Databases"
              helperText="Teleport will discover RDS databases and establish secure database connections
   through the Teleport proxy."
              checked={rdsConfig.enabled}
              onChange={toggleRds}
            />

            <Box ml={4} mb={3}>
              <FilterButton onClick={toggleRds} size="small">
                <Flex alignItems="center" gap={1}>
                  <FilterChevron size="small" expanded={rdsConfig.enabled} />
                  Filter by tag
                  <Icons.Info size="small" ml={1} />
                </Flex>
              </FilterButton>
              {rdsConfig.enabled && (
                <Box mt={1} mb={2} width={400}>
                  <LabelsInput
                    adjective="tag"
                    labels={rdsConfig.matchers as Label[]}
                    setLabels={(matchers: Label[]) =>
                      setRdsConfig({
                        ...rdsConfig,
                        matchers: matchers,
                      })
                    }
                  />
                </Box>
              )}
            </Box>

            <FieldCheckbox
              label="EKS Clusters"
              helperText="Teleport will discover EKS clusters and enable secure kubectl access through
  the Teleport proxy."
              checked={eksConfig.enabled}
              onChange={toggleEks}
            />

            <Box ml={4} mb={3}>
              <FilterButton onClick={toggleEks} size="small">
                <Flex alignItems="center" gap={1}>
                  <FilterChevron size="small" expanded={eksConfig.enabled} />
                  Filter by tag
                  <Icons.Info size="small" ml={1} />
                </Flex>
              </FilterButton>
              {eksConfig.enabled && (
                <Box mt={1} mb={2} width={400}>
                  <LabelsInput
                    adjective="tag"
                    labels={eksConfig.matchers as Label[]}
                    setLabels={(matchers: Label[]) =>
                      setEksConfig({
                        ...eksConfig,
                        matchers: matchers,
                      })
                    }
                  />
                </Box>
              )}
            </Box>

            <H2>Regions</H2>
            <Text mb={3}>
              Select the AWS regions where your resources are located.
            </Text>

            <Box>
              <FieldRadio
                name="regions"
                label={
                  <Flex alignItems="center" gap={2}>
                    <RadioLabel selected={allRegions}>All Regions</RadioLabel>
                  </Flex>
                }
                checked={allRegions}
                onChange={() => {
                  setAllRegions(true);
                  setAwsConfig({ ...awsConfig, regions: [] });
                }}
                mb={3}
              />
              <FieldRadio
                name="regions"
                label={
                  <Flex alignItems="center" gap={2}>
                    <RadioLabel selected={!allRegions}>
                      Select specific Regions
                    </RadioLabel>
                  </Flex>
                }
                checked={!allRegions}
                onChange={() => {
                  setAllRegions(false);
                  setAwsConfig({ ...awsConfig, regions: selectedRegions });
                }}
                mb={3}
              />

              {!allRegions && (
                <Box mb={3}>
                  <RegionMultiSelector
                    selectedRegions={selectedRegions}
                    onChange={regions => {
                      setSelectedRegions(regions);
                      setAwsConfig({ ...awsConfig, regions: regions });
                    }}
                    disabled={false}
                  />
                </Box>
              )}
            </Box>

            <H2>Deployment Method</H2>
            <Text mb={3}>
              Choose how you would like to deploy AWS IAM and Teleport resources
              used to discover resources.
            </Text>

            <Box>
              <FieldRadio
                name="deployment-method"
                label={
                  <Flex alignItems="center" gap={2}>
                    <RadioLabel selected={deploymentMethod === 'terraform'}>
                      Terraform (Recommended)
                    </RadioLabel>
                  </Flex>
                }
                helperText="Automatically provision IAM roles and policies using Infrastructure
  as Code."
                value="terraform"
                checked={deploymentMethod === 'terraform'}
                onChange={() => setDeploymentMethod('terraform')}
                mb={3}
              />

              {deploymentMethod === 'terraform' && (
                <Flex flexDirection="column" ml={5} gap={2}>
                  <Text>
                    Add the Teleport module to your Terraform configuration
                  </Text>
                  <Text>
                    Copy the module on the right and paste it into your
                    Terraform configuration.
                  </Text>
                  <TextSelectCopyMulti lines={[{ text: `terraform apply` }]} />
                  <FieldInput
                    autoFocus={true}
                    rule={requiredRoleArn}
                    value={integrationConfig.roleArn}
                    required={true}
                    label="Role ARN"
                    placeholder="arn:aws.*:iam::012345678901:role/name"
                    maxWidth={334}
                    mt={2}
                    onChange={e =>
                      setIntegrationConfig({
                        ...integrationConfig,
                        roleArn: e.target.value.trim(),
                      })
                    }
                  />
                </Flex>
              )}
              <FieldRadio
                name="deployment-method"
                label={
                  <Flex alignItems="center" gap={2}>
                    <RadioLabel
                      selected={deploymentMethod === 'cloudformation'}
                    >
                      CloudFormation
                    </RadioLabel>
                  </Flex>
                }
                helperText="Deploy required IAM resources using AWS CloudFormation templates."
                value="cloudformation"
                checked={deploymentMethod === 'cloudformation'}
                onChange={() => setDeploymentMethod('cloudformation')}
                mb={3}
              />

              {deploymentMethod === 'cloudformation' && (
                <Flex flexDirection="column" ml={5} mb={2} gap={2}>
                  <Text>CloudFormation deployment goes here...</Text>
                </Flex>
              )}

              <FieldRadio
                name="deployment-method"
                label={
                  <Flex alignItems="center" gap={2}>
                    <RadioLabel selected={deploymentMethod === 'manual'}>
                      Manual
                    </RadioLabel>
                  </Flex>
                }
                helperText="Manually create and configure the required IAM roles and policies."
                value="manual"
                checked={deploymentMethod === 'manual'}
                onChange={() => setDeploymentMethod('manual')}
                mb={3}
              />

              {deploymentMethod === 'manual' && (
                <Flex flexDirection="column" ml={5} mb={2} gap={2}>
                  <Text>Manual deployment goes here...</Text>
                </Flex>
              )}
            </Box>
          </Container>
        </Box>
      )}
    </Validation>
  );
}

const Container = styled(Flex)`
  border-radius: 8px;
  background: ${props => props.theme.colors.levels.elevated};

  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

const FilterButton = styled(ButtonText)`
  background: transparent;
  border: none;
  padding: 0;
  color: ${props => props.theme.colors.text.main};
  cursor: pointer;
  font: inherit;

  &:hover {
    color: ${props => props.theme.colors.text.main};
    background: transparent;
  }

  &:focus-visible {
    outline: 2px solid
      ${props => props.theme.colors.interactive.solid.primary.default};
    outline-offset: 2px;
  }
`;

const FilterChevron = styled(Icons.ChevronRight)<{ expanded: boolean }>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'rotate(90deg)' : 'none')};
`;

const RadioLabel = styled(Flex)<{ selected: boolean }>`
  font-weight: ${props => (props.selected ? '600' : 'inherit')};
`;
