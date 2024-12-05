/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Danger } from 'design/Alert';
import Flex from 'design/Flex';
import Indicator from 'design/Indicator';
import { useEffect } from 'react';
import { useAsync } from 'shared/hooks/useAsync';
import { useTheme } from 'styled-components';
import { H1 } from 'design/Text';
import Box from 'design/Box';
import { H3, P, P3 } from 'design/Text/Text';
import { ButtonSecondary } from 'design/Button';
import Image from 'design/Image';

import { State as ResourcesState } from 'teleport/components/useResources';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { RoleEditor } from './RoleEditor';
import tagpromo from './tagpromo.png';

/**
 * This component is responsible for converting from the `Resource`
 * representation of a role to a more accurate `RoleWithYaml` structure. The
 * conversion is asynchronous and it's performed on the server side.
 */
export function RoleEditorAdapter({
  resources,
  onSave,
  onDelete,
  onCancel,
}: {
  resources: ResourcesState;
  onSave: (role: Partial<RoleWithYaml>) => Promise<void>;
  onDelete: () => Promise<void>;
  onCancel: () => void;
}) {
  const theme = useTheme();
  const [convertAttempt, convertToRole] = useAsync(
    async (yaml: string): Promise<RoleWithYaml | null> => {
      if (resources.status === 'creating' || !resources.item) {
        return null;
      }
      return {
        yaml,
        object: await yamlService.parse<Role>(YamlSupportedResourceKind.Role, {
          yaml,
        }),
      };
    }
  );

  const originalContent = resources.item?.content ?? '';
  useEffect(() => {
    convertToRole(originalContent);
  }, [originalContent]);

  return (
    <Flex flex="1">
      <Flex
        flexDirection="column"
        borderLeft={1}
        borderColor={theme.colors.interactive.tonal.neutral[0]}
        backgroundColor={theme.colors.levels.surface}
        width="700px"
      >
        {convertAttempt.status === 'processing' && (
          <Flex
            flexDirection="column"
            alignItems="center"
            justifyContent="center"
            flex="1"
          >
            <Indicator />
          </Flex>
        )}
        {convertAttempt.status === 'error' && (
          <Danger>{convertAttempt.statusText}</Danger>
        )}
        {convertAttempt.status === 'success' && (
          <RoleEditor
            originalRole={convertAttempt.data}
            onCancel={onCancel}
            onSave={onSave}
            onDelete={onDelete}
          />
        )}
      </Flex>
      <Flex flex="1" alignItems="center" justifyContent="center">
        <Box>
          <H1 mb={2}>Teleport Policy</H1>
          <Flex mb={4}>
            <Box
              flex="1"
              css={`
                width: min-content;
              `}
            >
              <P>
                TAG serves as a powerful tool for both technical and security
                teams to maintain robust access control structures, ensuring
                security, compliance, and operational efficiency.
              </P>
            </Box>
            <Flex flex="0 0 auto" ml={6} alignItems="start">
              <ButtonLockedFeature noIcon py={0} width={undefined}>
                Contact Sales
              </ButtonLockedFeature>
              <ButtonSecondary
                as="a"
                href="https://goteleport.com/platform/policy/"
                target="_blank"
                ml={2}
              >
                Learn More
              </ButtonSecondary>
            </Flex>
          </Flex>
          <Flex
            flexDirection="column"
            bg={theme.colors.levels.surface}
            borderRadius={3}
          >
            <Box
              border={2}
              borderRadius={3}
              borderColor={theme.colors.interactive.tonal.neutral[0]}
            >
              {/* Note: image dimensions hardcoded to prevent UI glitches while
                  loading. */}
              <Image src={tagpromo} width={782} height={401} />
            </Box>
            <Box m={4}>
              <H3>Get clear Visualization of Access Relationships</H3>
              <Flex>
                <Box flex="1" width="min-content">
                  <P3>
                    TAG serves as a powerful tool for both technical and
                    security teams to maintain robust access control structures,
                    ensuring security, compliance, and operational efficiency.
                  </P3>
                </Box>
              </Flex>
            </Box>
          </Flex>
        </Box>
      </Flex>
    </Flex>
  );
}
