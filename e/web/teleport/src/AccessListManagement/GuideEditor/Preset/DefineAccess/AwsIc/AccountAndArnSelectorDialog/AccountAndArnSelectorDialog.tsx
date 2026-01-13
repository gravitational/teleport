import { useState } from 'react';

import { Alert, Box, Flex, H2, Indicator, Text } from 'design';
import Dialog from 'design/Dialog';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { UnifiedResourceApp } from 'shared/components/UnifiedResources';
import Validation from 'shared/components/Validation';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { NoResultsFound } from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { PermissionSetSelectorSection } from './PermissionSetSelectorSection';
import { footerHeight, OptionRow, OptionsContainer } from './Shared';
import { StickyFooter } from './StickyFooter';
import { SubmitSearchInput } from './SubmitSearchInput';
import { AccountOption, PermissionOption } from './types';

export function AccountAndArnSelectorDialog({ onClose }: { onClose(): void }) {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;

  const [searchAccountVal, setSearchAccountVal] = useState('');
  const [selectedAccounts, setSelectedAccounts] = useState<AccountOption[]>([]);
  const [wantWildcard, setWantWildcard] = useState(false);

  const [selectedSharedPerms, setSelectedSharedPerms] = useState<
    PermissionOption[]
  >([]);

  function toggleAcount(isSelected: boolean, app: UnifiedResourceApp) {
    // remove from selection
    // update shared permission set
    let newSharedPerms = [...selectedSharedPerms];
    let newSelectedAccounts = [...selectedAccounts];
    if (isSelected) {
      // remove from selection
      newSelectedAccounts = selectedAccounts.filter(
        acc => acc.value.name != app.name
      );
      if (newSelectedAccounts.length === 0) {
        newSharedPerms = [];
      }
    } else {
      // add to selection
      newSelectedAccounts.push({
        value: app,
        label: app.friendlyName
          ? `${app.friendlyName} (${app.name})`
          : app.name,
        selected: true,
      });

      newSharedPerms = selectedSharedPerms.filter(selectedPs =>
        app.permissionSets.some(ps => ps.arn === selectedPs.value.arn)
      );
    }

    setSelectedAccounts(newSelectedAccounts);
    setSelectedSharedPerms(newSharedPerms);
  }

  function addSelection() {
    const apps = selectedAccounts.map(opt => opt.value);
    const permSets = selectedSharedPerms.map(opt => opt.value);
    if (wantWildcard) {
      awsIcRoleState.addAccountWildcard(permSets);
    } else {
      awsIcRoleState.updateAccount(apps, permSets);
    }
    onClose();
  }

  let rows;
  if (awsIcRoleState.fetchedApps.isPending) {
    rows = (
      <Box textAlign="center" m={2}>
        <Indicator />
      </Box>
    );
  } else if (awsIcRoleState.fetchedApps.isError) {
    rows = (
      <Alert
        width="100%"
        primaryAction={{
          onClick: () => awsIcRoleState.fetchedApps.refetch(),
          content: 'Retry',
        }}
      >
        {awsIcRoleState.fetchedApps.error.message}
      </Alert>
    );
  } else if (awsIcRoleState.fetchedApps.isSuccess) {
    let filteredApps = [...awsIcRoleState.fetchedApps.data.list];

    if (searchAccountVal) {
      const search = searchAccountVal.toLocaleLowerCase();
      filteredApps = filteredApps.filter(
        app =>
          app.name.includes(search) ||
          app.friendlyName?.toLocaleLowerCase().includes(search)
      );
    }

    // Only allow first 100 items to render in dropdown, then let user use
    // the search bar to fine tune results. If the array has < 100 elements,
    // all items will just render.
    const filteredAppOptions = filteredApps.slice(0, 100).map(app => {
      const isSelected = selectedAccounts.some(
        acc => acc.value.name === app.name
      );
      return (
        <label key={app.name}>
          <OptionRow disabled={wantWildcard}>
            <FieldCheckbox
              checked={wantWildcard ? true : isSelected}
              mb={0}
              onChange={() => toggleAcount(isSelected, app)}
              disabled={wantWildcard}
            />
            <Box>
              <Text>{app.friendlyName}</Text>
              <Text fontSize={1} color="text.slightlyMuted">
                ID: {app.name}
              </Text>
            </Box>
          </OptionRow>
        </label>
      );
    });

    rows = filteredAppOptions;
    if (!searchAccountVal) {
      const wildcardOption = (
        <label key={'wildcard'}>
          <OptionRow>
            <FieldCheckbox
              checked={wantWildcard}
              mb={3}
              onChange={() => {
                setWantWildcard(!wantWildcard);
                // Wildcard should clear all selections
                // since it means any accounts and shared
                // perms have to be all re-calculated.
                setSelectedAccounts([]);
                setSelectedSharedPerms([]);
              }}
            />
            <Text>Select all (wildcard verb &quot;*&quot;)</Text>
          </OptionRow>
        </label>
      );

      rows = [wildcardOption, ...filteredAppOptions];
    }
  }

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '995px',
        width: '100%',
        padding: '20px',
      })}
      disableEscapeKeyDown={false}
      open={true}
    >
      <Validation>
        <Flex
          flexDirection="column"
          height="100%"
          maxHeight="600px"
          position="relative"
          minHeight={'60vh'}
        >
          <H2
            bold
            css={`
              min-height: 22px;
            `}
          >
            Select AWS Accounts and Permission Sets
          </H2>
          <Flex
            mt={4}
            gap={3}
            css={`
              flex-grow: 1; /* Takes up remaining space */
              overflow: hidden;
            `}
          >
            {/* AWS Account Selector Section */}
            <Box width="50%" overflow="hidden">
              <Text bold mb={2}>
                1. Select AWS Accounts
              </Text>
              <SubmitSearchInput
                placeholder="Search for AWS account"
                searchInputName="searchAccountVal"
                setSearchValue={setSearchAccountVal}
                defaultValue={searchAccountVal}
                disabled={awsIcRoleState.fetchedApps.isError}
              />
              {rows.length > 0 ? (
                <OptionsContainer>{rows}</OptionsContainer>
              ) : (
                <Flex alignItems="center" flexDirection="column" mt={3}>
                  <NoResultsFound />
                </Flex>
              )}
            </Box>

            <PermissionSetSelectorSection
              selectedAccounts={selectedAccounts}
              wantWildcard={wantWildcard}
              selectedSharedPerms={selectedSharedPerms}
              onChangeSharedPerms={setSelectedSharedPerms}
            />
          </Flex>
          {/* Filler so the absolutely positioned footer can take its place
        and sections meant to stay above the footer stays above it instead of
        behind the footer */}
          <Box minHeight={footerHeight}></Box>
          <StickyFooter
            onAddSelection={addSelection}
            onDialogClose={onClose}
            disabled={!selectedSharedPerms.length}
          />
        </Flex>
      </Validation>
    </Dialog>
  );
}
