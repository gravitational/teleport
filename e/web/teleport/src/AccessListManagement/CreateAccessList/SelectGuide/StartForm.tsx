import { useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonBorder,
  ButtonPrimary,
  Link as ExternalLink,
  Flex,
  Stack,
  Text,
} from 'design';
import { FieldRadio } from 'design/FieldRadio';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

export function StartForm({
  onStart,
  hasRoleAccess,
  missingRoleAccess,
  featureLimitReached,
}: {
  onStart(
    preset: AccessListPreset,
    accessListName: string,
    description: string
  ): void;
  hasRoleAccess: boolean;
  missingRoleAccess: string[];
  featureLimitReached: boolean;
}) {
  const [accessListName, setAccessListName] = useState('');
  const [description, setDescription] = useState('');
  const [selectedPreset, setSelectedPreset] =
    useState<AccessListPreset>('short-term');

  function handleOnStart(validator: Validator, preset?: AccessListPreset) {
    if (!validator.validate()) {
      return;
    }

    onStart(preset, accessListName, description);
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <Box width="535px" mb={4}>
            <FieldInput
              width="100%"
              label="Access List Name"
              rule={requiredField('Access List name must be specified')}
              value={accessListName}
              onChange={e => setAccessListName(e.target.value)}
              placeholder="Access List name"
              required
              autoFocus={!featureLimitReached}
            />
            <FieldTextArea
              label="Description"
              placeholder="Optional description about this Access List"
              value={description}
              onChange={e => setDescription(e.target.value)}
            />
          </Box>
          <Box>
            <Text bold>Select a guide</Text>
            <Stack mt={2} gap={2}>
              <HoverTooltip
                placement="right"
                tipContent={
                  <Box>
                    <Text mb={2}>
                      This guide helps you define which resources members of
                      this Access List can request. Members must submit an{' '}
                      <Text as="span" bold>
                        Access Request
                      </Text>{' '}
                      and receive approval before they can access those
                      resources.
                    </Text>
                    <Text>
                      Approved access is temporary. Its duration is based on the
                      member&apos;s request and limited by their remaining
                      Teleport session.
                    </Text>
                  </Box>
                }
              >
                <Box>
                  <FieldRadio
                    label={
                      <Box>
                        <RadioHeader>
                          Just-in-Time Access Guide{' '}
                          <Text as="span" bold>
                            (Recommended)
                          </Text>
                        </RadioHeader>
                        <RadioDescription>
                          List members will have to{' '}
                          <ExternalLink
                            target="_blank"
                            href="https://goteleport.com/docs/identity-governance/access-requests/"
                          >
                            request access
                          </ExternalLink>{' '}
                          to resources in this list.
                        </RadioDescription>
                      </Box>
                    }
                    size="small"
                    checked={selectedPreset === 'short-term'}
                    onChange={() => setSelectedPreset('short-term')}
                    mb={1}
                  />
                </Box>
              </HoverTooltip>
              <HoverTooltip
                placement="right"
                tipContent={
                  <Box>
                    <Text>
                      This guide helps you define which resources members of
                      this Access List can access without submitting an Access
                      Request.
                    </Text>
                    <Text mt={3}>
                      Access is granted automatically when members sign in and
                      remains active for the duration of their Teleport session.
                    </Text>
                  </Box>
                }
              >
                <Box>
                  <FieldRadio
                    label={
                      <Box>
                        <RadioHeader>Standing Access Guide</RadioHeader>
                        <RadioDescription>
                          List members will get standing access to resources in
                          this list.
                        </RadioDescription>
                      </Box>
                    }
                    size="small"
                    checked={selectedPreset === 'long-term'}
                    onChange={() => setSelectedPreset('long-term')}
                  />
                </Box>
              </HoverTooltip>
            </Stack>
          </Box>

          <Flex gap={3} mt={3} alignItems={'center'}>
            <HoverTooltip
              tipContent={
                !hasRoleAccess ? (
                  <Box>
                    You cannot use this guide. Missing role permissions:{' '}
                    <code>{missingRoleAccess.join(', ')}</code>
                  </Box>
                ) : undefined
              }
            >
              <ButtonPrimary
                onClick={() => handleOnStart(validator, selectedPreset)}
                width="150px"
                disabled={!hasRoleAccess}
              >
                Start Guide
              </ButtonPrimary>
            </HoverTooltip>
            <ButtonBorder
              intent="primary"
              onClick={() => handleOnStart(validator, '')}
              width="220px"
            >
              Use Custom Form Instead
            </ButtonBorder>
          </Flex>
        </>
      )}
    </Validation>
  );
}

const RadioHeader = styled(Text)`
  font-size: ${p => p.theme.fontSizes[3]}px;
`;

const RadioDescription = styled(Box)`
  color: ${p => p.theme.colors.text.slightlyMuted};
  width: 340px;
`;
