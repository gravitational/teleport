import { useMemo, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { NoResultsFound } from 'e-teleport/AccessListManagement/GuideEditor/Shared';
import { PermissionSet } from 'teleport/services/apps';

import { PermissionSetEmptyState } from './PermissionSetEmptyState';
import { OptionRow, OptionsContainer } from './Shared';
import { SubmitSearchInput } from './SubmitSearchInput';
import { AccountOption, PermissionOption } from './types';

export function PermissionSetSelectorSection({
  selectedAccounts,
  wantWildcard,
  selectedSharedPerms,
  onChangeSharedPerms,
}: {
  selectedAccounts: AccountOption[];
  selectedSharedPerms: PermissionOption[];
  onChangeSharedPerms(newOpts: PermissionOption[]): void;
  wantWildcard: boolean;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;

  const [searchPermVal, setSearchPermVal] = useState('');

  const sharedPermsBtwnSelectedApps = useMemo(() => {
    let selectedApps = [...selectedAccounts];

    // Wildcard loads all existing apps for calculation.
    if (wantWildcard) {
      selectedApps = awsIcRoleState.fetchedApps.data.list.map(app => ({
        value: app,
        label: app.friendlyName
          ? `${app.friendlyName} (${app.name})`
          : app.name,
        selected: true,
      }));
    }

    // Stores list of accounts that belong to an arn
    // And stores other data related to that arn
    const arnLookup: Record<
      string, // arn
      { accounts: string[]; permissionSet: PermissionSet }
    > = {};

    // Go through each app and build the arnLookup.
    selectedApps.forEach(app =>
      app.value.permissionSets.forEach(ps => {
        const exists = arnLookup[ps.arn];
        if (exists) {
          arnLookup[ps.arn] = {
            accounts: [...exists.accounts, app.value.name],
            permissionSet: ps,
          };
        } else {
          arnLookup[ps.arn] = {
            accounts: [app.value.name],
            permissionSet: ps,
          };
        }
      })
    );

    const selectedAppNames = selectedApps.map(app => app.value.name);
    const sharedPerms: PermissionOption[] = [];

    // Calculate shared arns between apps.
    Object.keys(arnLookup).forEach(arn => {
      const thisArn = arnLookup[arn];
      if (
        thisArn.accounts.length === selectedApps.length &&
        thisArn.accounts.every(acc => selectedAppNames.includes(acc))
      ) {
        sharedPerms.push({
          value: thisArn.permissionSet,
          label: thisArn.permissionSet.name,
          selected: false,
        });
      }
    });

    return sharedPerms;
  }, [selectedAccounts, wantWildcard, awsIcRoleState.fetchedApps.data?.list]);

  const filteredSharedPerms = useMemo(() => {
    let filtered = [...sharedPermsBtwnSelectedApps];
    if (searchPermVal) {
      const search = searchPermVal.toLocaleLowerCase();
      filtered = filtered.filter(({ value }) =>
        value.name.toLocaleLowerCase().includes(search)
      );
    }
    return filtered;
  }, [sharedPermsBtwnSelectedApps, searchPermVal]);

  function onChangeCheckbox(isSelected: boolean, perm: PermissionOption) {
    // remove from selection
    // update shared permission set
    if (isSelected) {
      onChangeSharedPerms(
        selectedSharedPerms.filter(acc => acc.value.arn != perm.value.arn)
      );
      return;
    }
    // add to selection
    onChangeSharedPerms([
      ...selectedSharedPerms,
      {
        value: perm.value,
        label: perm.value.name,
        selected: true,
      },
    ]);
  }

  return (
    <Box width="50%">
      {sharedPermsBtwnSelectedApps?.length === 0 && (
        <PermissionSetEmptyState
          selectedAccounts={selectedAccounts}
          hasWildCard={wantWildcard}
        />
      )}
      {sharedPermsBtwnSelectedApps?.length > 0 && (
        <PermissionSetSection>
          <Text bold mb={2}>
            2. Select Permission Sets
          </Text>
          <SubmitSearchInput
            placeholder="Search for AWS permissions"
            searchInputName="searchPermVal"
            setSearchValue={setSearchPermVal}
            defaultValue={searchPermVal}
          />
          <OptionsContainer>
            {!filteredSharedPerms.length && (
              <Flex alignItems="center" flexDirection="column" mt={3}>
                <NoResultsFound />
              </Flex>
            )}
            {filteredSharedPerms.map(perm => {
              const isSelected = selectedSharedPerms.some(
                acc => acc.value.arn === perm.value.arn
              );
              return (
                <label key={perm.value.arn}>
                  <OptionRow>
                    <FieldCheckbox
                      checked={isSelected}
                      mb={0}
                      onChange={() => onChangeCheckbox(isSelected, perm)}
                    />
                    <Text>{perm.value.name}</Text>
                  </OptionRow>
                </label>
              );
            })}
          </OptionsContainer>
        </PermissionSetSection>
      )}
    </Box>
  );
}

const PermissionSetSection = styled(Box)`
  padding-left: ${p => p.theme.space[3]}px;
  overflow: hidden;
  height: 100%;
  border-left: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;
