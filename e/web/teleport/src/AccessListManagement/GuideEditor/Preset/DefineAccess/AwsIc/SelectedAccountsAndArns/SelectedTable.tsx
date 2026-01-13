import styled from 'styled-components';

import { Box, ButtonIcon, Flex, Text } from 'design';
import { Cross } from 'design/Icon';
import { LabelButtonWithIcon } from 'design/Label/LabelButtonWithIcon';
import { HoverTooltip } from 'design/Tooltip';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';

import { AwsIcApp } from '../../../role/conditions';
import { wildCard } from '../../../role/role';

export function SelectedTable({
  paginatedApps,
  currentPage,
}: {
  currentPage: number;
  paginatedApps: AwsIcApp[][];
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;

  return (
    <OuterBoxGrid mt={4}>
      <Text bold pl={2}>
        AWS Account
      </Text>

      <Text bold>Permission Sets</Text>

      {/* fill in element for the third column that
        renders delete icon for the rows */}
      <Text></Text>

      {paginatedApps[currentPage]?.map(app => {
        let accountName = app.friendlyAccountName || app.accountId;
        if (app.accountId === wildCard) {
          accountName = 'Any Account (wildcard "*")';
        }

        return (
          <InnerBoxGrid
            key={app.accountId}
            data-testid={`row-${app.accountId}`}
          >
            <HoverTooltip tipContent={accountName}>
              <Box pr={4}>
                <Text>{accountName}</Text>
                {app.friendlyAccountName && (
                  <Text color="text.slightlyMuted" fontSize={1}>
                    ID: {app.accountId}
                  </Text>
                )}
              </Box>
            </HoverTooltip>
            <Flex gap={2} alignSelf="anchor-center" flexWrap="wrap">
              {[...app.arnMap.keys()].map(arnId => {
                return (
                  <LabelButtonWithIcon
                    key={arnId}
                    kind="secondary"
                    IconRight={Cross}
                    onClick={() =>
                      awsIcRoleState.removeArn(app.accountId, arnId)
                    }
                    withHoverState={true}
                  >
                    <Text>{app.arnMap.get(arnId) || arnId}</Text>
                  </LabelButtonWithIcon>
                );
              })}
            </Flex>
            <Flex alignItems="center">
              <ButtonIcon
                onClick={() => awsIcRoleState.removeAccount(app.accountId)}
                data-testid={`remove-${app.accountId}`}
              >
                <Cross size="small" color={'text.slightlyMuted'} />
              </ButtonIcon>
            </Flex>
          </InnerBoxGrid>
        );
      })}
    </OuterBoxGrid>
  );
}

const OuterBoxGrid = styled(Box)`
  display: grid;
  grid-template-columns: 23.5% 73.5% 3%;
  width: 100%;
`;

const InnerBoxGrid = styled(Box)`
  grid-column: span 3;
  display: grid;
  grid-template-columns: subgrid;
  padding: ${p => p.theme.space[2]}px;
  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }
  border-bottom: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;
