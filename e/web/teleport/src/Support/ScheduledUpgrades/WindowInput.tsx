import { UseMutationResult } from '@tanstack/react-query';
import { Dispatch, SetStateAction } from 'react';

import { Box, ButtonBorder, ButtonText, Flex, Text } from 'design';
import { Check, Pencil } from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';
import Select, { Option } from 'shared/components/Select';

import {
  availableUpgradeWindowStartHours,
  UpgradeWindowStartHour,
} from 'e-teleport/services/cloud';
import { EditableInput } from 'e-teleport/Support/ScheduledUpgrades/EditableInput';

const makeLabel = (startHour: UpgradeWindowStartHour): string => {
  return `${String(startHour).padStart(2, '0')}:00 (UTC)`;
};

export function WindowInput({
  mutation,
  edit,
  setEdit,
  setValue,
  value,
  muted = false,
}: {
  mutation: UseMutationResult<UpgradeWindowStartHour, Error, void, unknown>;
  edit: boolean;
  setEdit: Dispatch<SetStateAction<boolean>>;
  value: UpgradeWindowStartHour;
  setValue: Dispatch<SetStateAction<UpgradeWindowStartHour>>;
  muted?: boolean;
}) {
  return (
    <EditableInput
      muted={muted}
      title="Window Start Time"
      error={mutation?.error}
      content={
        <Flex alignItems="center" gap="2">
          {edit ? (
            <>
              <Select
                size="small"
                css={{ minWidth: '125px' }}
                options={availableUpgradeWindowStartHours.map(p => ({
                  value: p.toString(),
                  label: makeLabel(p),
                }))}
                onChange={(opt: Option) => {
                  if (!opt || !opt.value) {
                    return;
                  }
                  setValue(+opt.value as UpgradeWindowStartHour);
                }}
                value={{
                  value: value.toString(),
                  label: makeLabel(value),
                }}
                isSearchable={false}
                isDisabled={mutation.isPending}
              />
              <ButtonBorder
                title="Save"
                py="0"
                px="2"
                onClick={() => mutation.mutate()}
                intent="primary"
                disabled={mutation.isPending}
              >
                <Check size="small" />
              </ButtonBorder>
              {mutation.error && (
                <Box
                  data-testid={'window-warn-box'}
                  // use visibility to prevent layout shift when the tooltip appears
                  style={{
                    visibility: mutation?.error ? 'visible' : 'hidden',
                  }}
                >
                  <IconTooltip kind="error">
                    {mutation?.error?.message}
                  </IconTooltip>
                </Box>
              )}
            </>
          ) : (
            <>
              <Text>{makeLabel(value)}</Text>
              <ButtonText
                title="Edit"
                py="0"
                px="2"
                onClick={() => {
                  // clear previous error
                  mutation.reset();
                  setEdit(true);
                }}
              >
                <Pencil size="small" />
              </ButtonText>
            </>
          )}
        </Flex>
      }
    />
  );
}
