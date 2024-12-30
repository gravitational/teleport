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

import React, { useEffect } from 'react';
import { useTheme } from 'styled-components';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { ChevronLeft, ChevronRight } from 'design/Icon';
import Image from 'design/Image';
import { Indicator } from 'design/Indicator';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import { H1 } from 'design/Text';
import { H3, P, P3 } from 'design/Text/Text';
import { useAsync } from 'shared/hooks/useAsync';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { State as ResourcesState } from 'teleport/components/useResources';
import cfg from 'teleport/config';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

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
  onCancel,
}: {
  resources: ResourcesState;
  onSave: (role: Partial<RoleWithYaml>) => Promise<void>;
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
          />
        )}
      </Flex>
      <Flex flex="1" alignItems="center" justifyContent="center" m={3}>
        {/* Same width as promo image + border */}
        <Box maxWidth={promoImageWidth + 2 * 2} minWidth={300}>
          <H1 mb={2}>Coming soon: Teleport Policy saves you from mistakes</H1>
          <Flex mb={4} gap={4} flexWrap="wrap" justifyContent="space-between">
            <Box flex="1" minWidth="30ch">
              <P>
                Teleport Policy will visualize resource access paths as you
                create and edit roles so you can always see what you are
                granting before you push a role into production.
              </P>
            </Box>
            <Flex flex="0 0 auto" alignItems="start">
              {!cfg.isPolicyEnabled && (
                <>
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
                </>
              )}
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
              <Image src={tagpromo} width="100%" />
            </Box>
            <StepSlider
              flows={promoFlows}
              currFlow={
                resources.status === 'creating' ? 'creating' : 'updating'
              }
            />
          </Flex>
        </Box>
      </Flex>
    </Flex>
  );
}

const promoImageWidth = 782;

const promoFlows = {
  creating: [VisualizeAccessPathsPanel, VisualizeDiffPanel],
  updating: [VisualizeDiffPanel, VisualizeAccessPathsPanel],
};

function VisualizeAccessPathsPanel(props: StepComponentProps) {
  return (
    <PromoPanel
      {...props}
      heading="Visualize access paths granted by your roles"
      content={
        <>
          See what you’re granting before pushing to prod. Teleport Policy will
          show resource access paths granted by your role before you save
          changes.
        </>
      }
    />
  );
}

function VisualizeDiffPanel(props: StepComponentProps) {
  return (
    <PromoPanel
      {...props}
      heading="Visualize the diff in permissions as you edit roles"
      content={
        <>
          Prevent mistakes. Teleport Policy shows you what access is removed and
          what is added as you make edits to a role—all before you save your
          changes.
        </>
      }
    />
  );
}

function PromoPanel({
  prev,
  next,
  refCallback,
  stepIndex,
  flowLength,
  heading,
  content,
}: StepComponentProps & {
  heading: React.ReactNode;
  content: React.ReactNode;
}) {
  return (
    <Flex m={4} gap={4} ref={refCallback}>
      <Box flex="1">
        <H3>{heading}</H3>
        <Box flex="1">
          <P3>{content}</P3>
        </Box>
      </Box>
      <Flex gap={2} alignItems="center">
        <ButtonSecondary size="small" width="24px" disabled={stepIndex <= 0}>
          <ChevronLeft size="small" onClick={prev} />
        </ButtonSecondary>
        <ButtonSecondary
          size="small"
          width="24px"
          disabled={stepIndex >= flowLength - 1}
        >
          <ChevronRight size="small" onClick={next} />
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
