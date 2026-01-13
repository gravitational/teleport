import { JSX, useMemo, useState } from 'react';

import { Box, Button, Flex, H2, Image, Indicator, Text } from 'design';
import { Plus } from 'design/Icon';
import { awsiamidentitycenter } from 'design/ResourceIcon/icons';
import { HoverTooltip } from 'design/Tooltip';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';

import { AwsIcApp } from '../../role/conditions';
import { wildCard } from '../../role/role';
import { EmptyList } from '../EmptyList';
import { AccountAndArnSelectorDialog } from './AccountAndArnSelectorDialog/AccountAndArnSelectorDialog';
import { SelectedAccountsAndArns } from './SelectedAccountsAndArns/SelectedAccountsAndArns';

export function AwsIcSection() {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;

  const [showAccAndArnSelector, setShowAccAndArnSelector] = useState(false);

  /**
   * List of aws ic apps loaded with friendly names if possible.
   */
  const selectedApps = useMemo(() => {
    const selectedApps: AwsIcApp[] = [];
    for (const [accountId, arns] of awsIcRoleState.roleConditions.account) {
      const arnMap = new Map();
      const app = awsIcRoleState.fetchedApps.data?.lookup.get(accountId);

      // Try loading friendly names for permission arns.
      arns?.forEach(arn => {
        const arnFriendlyName =
          awsIcRoleState.fetchedApps.data?.permissionSet.get(arn);
        arnMap.set(arn, arnFriendlyName);
      });

      selectedApps.push({
        accountId: accountId,
        friendlyAccountName: app?.friendlyName,
        arnMap,
      });
    }
    return selectedApps;
  }, [awsIcRoleState.fetchedApps, awsIcRoleState.roleConditions]);

  const hasWildCard =
    selectedApps.length === 1 && selectedApps[0].accountId === wildCard;

  const addButton = (
    <AddButton
      hasWildCard={hasWildCard}
      hasSelectedAccounts={awsIcRoleState.roleConditions.account.size > 0}
      onClickAdd={() => setShowAccAndArnSelector(true)}
    />
  );

  const header = (
    <H2 bold mt={2} mb={4}>
      Define AWS Identity Center Access
    </H2>
  );

  if (awsIcRoleState.fetchedApps.isPending) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  let content: JSX.Element;
  if (selectedApps.length > 0) {
    content = (
      <>
        {header}
        <SelectedAccountsAndArns selectedApps={selectedApps} />
        {addButton}
      </>
    );
  } else {
    // no apps selected

    // fetchedAwsIcApps can also possibly error but the error is checked where
    // the data is required which is in AwsIcSelectorDialog. However loading
    // is checked earlier to see if we can extract friendly names which isn't
    // strictly required since its ID can be used as fallback.
    if (awsIcRoleState.fetchedApps.data?.list.length === 0) {
      // CTA to add AWS IC
      content = <EmptyList resourceField="awsIc" />;
    } else {
      content = (
        <>
          {header}
          <NoSelectionsMade />
          {addButton}
        </>
      );
    }
  }

  return (
    <Box>
      {showAccAndArnSelector ? (
        <AccountAndArnSelectorDialog
          onClose={() => setShowAccAndArnSelector(false)}
        />
      ) : (
        content
      )}
    </Box>
  );
}

function AddButton({
  hasWildCard,
  hasSelectedAccounts,
  onClickAdd,
}: {
  hasWildCard: boolean;
  hasSelectedAccounts: boolean;
  onClickAdd(): void;
}) {
  return (
    <Flex flexDirection="column" alignItems="center">
      <HoverTooltip
        tipContent={hasWildCard ? `To edit, remove wildcard.` : undefined}
      >
        <Button
          onClick={onClickAdd}
          mt={3}
          intent="primary"
          size="medium"
          fill={hasSelectedAccounts ? 'border' : 'filled'}
          disabled={hasWildCard}
        >
          <Flex gap="2">
            <Plus size="small" />
            Make a new selection
          </Flex>
        </Button>
      </HoverTooltip>
    </Flex>
  );
}

function NoSelectionsMade() {
  return (
    <Flex
      flexDirection="column"
      alignItems="center"
      css={`
        border-bottom: 1px solid
          ${p => p.theme.colors.interactive.tonal.neutral[0]};
      `}
      pb={5}
      mt={8}
      mb={4}
      gap={2}
    >
      <Flex alignItems="center" gap={2}>
        <Image src={awsiamidentitycenter} width={40} height={40} />
        <Text bold fontSize={7}>
          No Access Defined
        </Text>
      </Flex>
      <Box width="280px" textAlign="center" color="text.slightlyMuted">
        To configure access, add AWS Accounts and Permission Sets below.
      </Box>
    </Flex>
  );
}
