import { useLocation, useNavigate } from 'react-router';

import { Box, Text, ButtonPrimary, ButtonSecondary, H2 } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import {
  AccessListOwners,
  ConfigureFilters,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Entra/SyncSettings/GroupsImport';
import { SyncIntervals } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Entra/SyncSettings/SyncIntervals';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import cfg from 'teleport/config';
import { StyledBox } from 'teleport/Discover/Shared';
import type {
  Plugin,
  PluginEntraIdSyncIntervals,
} from 'teleport/services/integrations';

import { Filters } from '../types';
import { emptyFilter } from './constants';
import { useSyncSettings } from './useSyncSettings';

export function SyncSettings({
  plugin,
  onSave,
  disabled,
}: {
  plugin?: Plugin;
  onSave: (
    filters: Filters,
    owners: string[],
    ownersSource: string,
    syncIntervals: PluginEntraIdSyncIntervals
  ) => void;
  disabled: boolean;
}) {
  const navigate = useNavigate();
  const location = useLocation();

  const {
    importAll,
    setImportAll,
    filters,
    setFilters,
    selectedOwners,
    setSelectedOwners,
    accessListOwnersSource,
    setAccessListOwnersSource,
    syncIntervals,
    setSyncIntervals,
  } = useSyncSettings(plugin);

  function save(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    let filterValue = emptyFilter;
    if (!importAll) {
      // Toggle on (import all) state should wipe out
      // filters because the "import all" behavior
      // requires zero configured filters.
      filterValue = filters;
    }

    onSave(
      filterValue,
      selectedOwners.map(o => o.label),
      accessListOwnersSource,
      syncIntervals
    );
  }

  function goBack() {
    if (!location.key || location.key === 'default') {
      navigate(cfg.getIntegrationStatusRoute('entra-id', plugin.name));
    } else {
      navigate(-1);
    }
  }

  return (
    <Box mt={3} width="800px">
      <Header header="Edit Sync Settings" />
      <Text mt={2}>
        Edit Entra ID plugin sync settings. Changes will be applied in the next
        import cycle.
      </Text>

      <Validation>
        {({ validator }) => (
          <>
            <H2 mb={2} mt={5}>
              Sync Intervals
            </H2>
            <Text>Configure sync mode and frequency.</Text>
            <StyledBox mt={3}>
              <SyncIntervals
                syncIntervals={syncIntervals}
                onIntervalChange={setSyncIntervals}
                disabled={disabled}
              />
            </StyledBox>

            <H2 mb={2} mt={5}>
              Groups Import
            </H2>
            <Text>
              Groups imported from the Microsoft Entra ID directory will be
              created as Access Lists <br /> and their respective group members
              will be created as Access List members.
            </Text>

            <StyledBox mt={3}>
              <ConfigureFilters
                enabled={importAll}
                setEnabled={setImportAll}
                filters={filters}
                onFilterChange={setFilters}
                validator={validator}
                disabled={disabled}
              />
            </StyledBox>

            <StyledBox mt={4}>
              <AccessListOwners
                selectedOptions={selectedOwners}
                onOptionChange={setSelectedOwners}
                accessListOwnersSource={accessListOwnersSource}
                onOwnersSourceChange={setAccessListOwnersSource}
                disabled={disabled}
              />
            </StyledBox>

            <Box mt={6} mb={6}>
              <ButtonPrimary onClick={() => save(validator)} mr={3}>
                Save
              </ButtonPrimary>
              <ButtonSecondary onClick={goBack}>Back</ButtonSecondary>
            </Box>
          </>
        )}
      </Validation>
    </Box>
  );
}
