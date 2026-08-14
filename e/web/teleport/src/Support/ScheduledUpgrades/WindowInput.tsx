import {
  Box,
  Button,
  ButtonText,
  Flex,
  Text,
  Tooltip,
} from '@gravitational/design-system';
import { UseMutationResult } from '@tanstack/react-query';
import { Dispatch, SetStateAction } from 'react';

import { Check, Pencil } from 'design/Icon';
import Select, { Option } from 'shared/components/Select';

import {
  availableUpgradeWindowStartHours,
  UpgradeWindowStartHour,
} from 'e-teleport/services/cloud';
import {
  EditableInput,
  ErrorTooltipIcon,
} from 'e-teleport/Support/ScheduledUpgrades/EditableInput';

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
        <Flex align="center" gap="2">
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
              <Button
                title="Save"
                py="0"
                px="2"
                onClick={() => mutation.mutate()}
                fill="border"
                intent="primary"
                disabled={mutation.isPending}
              >
                <Check size="small" />
              </Button>
              {mutation.error && (
                <Box data-testid={'window-warn-box'}>
                  <Tooltip content={mutation?.error?.message}>
                    <ErrorTooltipIcon />
                  </Tooltip>
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
