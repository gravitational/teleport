import styled, { useTheme } from 'styled-components';

import { Box, Flex, H4 } from 'design';
import { LabelContent } from 'design/LabelInput/LabelInput';
import FieldInput from 'shared/components/FieldInput';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { LabelsInput } from 'teleport/components/LabelsInput';
import { Label } from 'teleport/types';

export function ReadAwsIcAccessSection() {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;

  const theme = useTheme();

  const labels = awsIcRoleState.roleConditions.labels;

  const labelInputs: Label[] = Object.keys(labels).map(labelKey => {
    if (Array.isArray(labels[labelKey])) {
      return {
        name: labelKey,
        value: labels[labelKey].join(', '),
      };
    }
    return {
      name: labelKey,
      value: labels[labelKey],
    };
  });

  return (
    <Flex flexDirection="column" gap={3}>
      <LabelsInput
        atLeastOneRow
        legend="Labels"
        labels={labelInputs}
        setLabels={() => null}
        readOnly
      />
      <Flex flexWrap="wrap" gap={3}>
        {Array.from(awsIcRoleState.roleConditions.account.entries()).map(
          ([accountId, arns]) => (
            <Box
              key={accountId}
              border={1}
              borderColor={theme.colors.interactive.tonal.neutral[0]}
              borderRadius={3}
              padding={3}
              width="100%"
            >
              <H4 mb={3}>Account Assignment</H4>
              <FieldInput
                label="AWS Account ID"
                value={accountId}
                readonly={true}
              />
              <LabelContent>Permission Sets</LabelContent>
              <Flex mt={1} gap={2} flexWrap="wrap">
                {[...arns].map(arn => (
                  <PermissionSet key={arn} py={2} px={3}>
                    {arn}
                  </PermissionSet>
                ))}
              </Flex>
            </Box>
          )
        )}
      </Flex>
    </Flex>
  );
}

const PermissionSet = styled(Box)`
  border-radius: ${props => props.theme.radii[2]}px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  width: 100%;
`;
