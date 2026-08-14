import styled from 'styled-components';

import { Box, H2, Stack, Text } from 'design';

import { CollapsiblePills, Pills } from './Pills';
import { OutlineBox, SmallHeader } from './Shared';

export const SelectedUsersSection = ({
  title,
  kind,
  selectedUsers,
  requiredRoles,
  requiredTraits,
}: {
  title: string;
  kind: 'owners' | 'members';
  selectedUsers: { label: string }[];
  requiredRoles: { label: string }[];
  requiredTraits: { name: string; value: string }[];
}) => {
  if (selectedUsers.length === 0) {
    return null;
  }

  const hasRequirements = requiredRoles.length > 0 || requiredTraits.length > 0;

  return (
    <Stack>
      <H2>{title}</H2>
      <OutlineBox>
        <CollapsiblePills texts={selectedUsers.map(m => m.label)} />
        {hasRequirements && (
          <Box mt={3}>
            <SmallHeader>
              Required roles and traits to be eligible {kind}
            </SmallHeader>
            {requiredRoles.length > 0 && (
              <Box mt={1}>
                <SubHeader>Roles</SubHeader>
                <Pills texts={requiredRoles.map(r => r.label)} />
              </Box>
            )}
            {requiredTraits.length > 0 && (
              <Box mt={1}>
                <SubHeader>Traits</SubHeader>
                <Pills
                  texts={requiredTraits.map(t => `${t.name}: ${t.value}`)}
                />
              </Box>
            )}
          </Box>
        )}
      </OutlineBox>
    </Stack>
  );
};

const SubHeader = styled(Text)`
  font-weight: ${p => p.theme.fontWeights.bold};
  font-size: ${p => p.theme.fontSizes[0]}px;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;
