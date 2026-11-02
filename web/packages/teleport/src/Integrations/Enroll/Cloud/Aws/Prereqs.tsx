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

import { Box, Flex, H2, H3, Text, styled } from '@gravitational/design-system';
import { useState } from 'react';

import { Link } from 'design';
import { ArrowSquareOut } from 'design/Icon';

import { TagsButton, TagsChevron } from './Tags';

export function Prereqs() {
  const [expand, setExpand] = useState(true);

  return (
    <Box>
      <TagsButton onClick={() => setExpand(prev => !prev)}>
        <Flex alignItems="center" gap={1}>
          <TagsChevron size="medium" expanded={expand} />
          <H2>Prerequisites</H2>
        </Flex>
      </TagsButton>
      {expand && (
        <Box mb={2} px={5}>
          <Text mb={1}>Before you begin, ensure the following are set up</Text>

          <H3>Terraform Providers</H3>
          <Container>
            <Link
              target="_blank"
              href="https://goteleport.com/docs/configuration/terraform-provider/"
            >
              <Flex alignItems={'center'}>
                Teleport Provider Configuration <ArrowSquareOut size="small" />
              </Flex>
            </Link>
            <DescriptionText>
              - sets up the integration in your Teleport cluster.
            </DescriptionText>
          </Container>

          <Container>
            <Link
              target="_blank"
              href="https://registry.terraform.io/providers/hashicorp/aws/latest/docs"
            >
              <Flex alignItems={'center'}>
                AWS Provider Configuration <ArrowSquareOut size="small" />
              </Flex>
            </Link>
            <DescriptionText>
              - sets up the IAM roles in your AWS account.
            </DescriptionText>
          </Container>

          <Box
            my={3}
            borderBottom="2px solid"
            borderColor="interactive.tonal.neutral.0"
          />

          <H3>AWS Permissions</H3>
          <Container>
            <Link
              target="_blank"
              href="https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/ec2-discovery-terraform#step-15-configure-aws-terraform-provider"
            >
              <Flex alignItems={'center'}>
                AWS IAM permissions for discovery{' '}
                <ArrowSquareOut size="small" />
              </Flex>
            </Link>
            <DescriptionText>
              - needed to find resources in your account.
            </DescriptionText>
          </Container>

          <Container>
            <Link
              target="_blank"
              href="https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html"
            >
              <Flex alignItems={'center'}>
                SSM agent running on EC2 instances{' '}
                <ArrowSquareOut size="small" />
              </Flex>
            </Link>
            <DescriptionText>
              - needed to install Teleport on your instances.
            </DescriptionText>
          </Container>
        </Box>
      )}
    </Box>
  );
}

const DescriptionText = styled(Text, {
  base: {
    textStyle: 'body3',
    fontStyle: 'italic',
  },
});

const Container = styled(Flex, {
  base: {
    gap: 1,
    alignItems: 'center',
  },
});
