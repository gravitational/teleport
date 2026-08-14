import { Box, Flex, H2, Stack, Text } from 'design';

import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

import { Spec } from '../types';
import { OutlineBox } from './Shared';

export function BasicInfoSection({
  spec,
  preset,
  hideBottomBorder,
}: {
  spec: Spec;
  preset: AccessListPreset;
  hideBottomBorder: boolean;
}) {
  function getAccessPrivilegeType() {
    switch (preset) {
      case 'short-term':
        return 'Just-in-Time Access';
      case 'long-term':
        return 'Standing Access';
      default:
        return '';
    }
  }

  function getAccessPrivilegeDesc() {
    switch (preset) {
      case 'short-term':
        return '* List members will have to request just-in-time access to resources in this list.';
      case 'long-term':
        return '';
      default:
        return '';
    }
  }

  return (
    <Stack>
      <H2>Basic Information</H2>

      <OutlineBox gap={2} $hideBottomBorder={hideBottomBorder}>
        <Flex justifyContent={'space-between'} gap={3}>
          <Box>
            <Text bold>Access List Name</Text>
            <Text>{spec.title}</Text>
          </Box>

          {preset && (
            <Box width="240px">
              <Text bold>Access Privilege</Text>
              <Text>{getAccessPrivilegeType()}</Text>
              <Text fontSize={1} color="text.slightlyMuted">
                {getAccessPrivilegeDesc()}
              </Text>
            </Box>
          )}
        </Flex>

        {spec.description && (
          <Box>
            <Text bold>Description</Text>
            <Text>{spec.description}</Text>
          </Box>
        )}

        <Box>
          <Text bold>Review Frequency:</Text>
          <Text>
            {spec.reviewFrequency.label}, {spec.reviewDayOfMonth.label}
          </Text>
        </Box>
      </OutlineBox>
    </Stack>
  );
}
