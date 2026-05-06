import { UseMutationResult } from '@tanstack/react-query';
import { Dispatch, SetStateAction } from 'react';

import { Box, ButtonBorder, ButtonText, Flex, Text } from 'design';
import { Check, Pencil } from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';
import Select, { Option } from 'shared/components/Select';

import { availableEnvironmentProfiles } from 'e-teleport/services/cloud';
import { EnvironmentProfile } from 'e-teleport/services/cloud/cloud';
import { GetEnvironmentProfileResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import { EditableInput } from 'e-teleport/Support/ScheduledUpgrades/EditableInput';

export function ProfileInput({
  mutation,
  edit,
  setEdit,
  setValue,
  value,
  muted = false,
}: {
  mutation: UseMutationResult<
    GetEnvironmentProfileResponse,
    Error,
    void,
    unknown
  >;
  edit: boolean;
  setEdit: Dispatch<SetStateAction<boolean>>;
  value: string;
  setValue: Dispatch<SetStateAction<EnvironmentProfile | undefined>>;
  muted?: boolean;
}) {
  return (
    <EditableInput
      muted={muted}
      title="Environment Profile"
      error={mutation?.error}
      content={
        <Flex alignItems="center" gap="2">
          {edit ? (
            <>
              <Select
                size="small"
                css={{ minWidth: '125px' }}
                options={availableEnvironmentProfiles.map(p => ({
                  value: p,
                  label: p,
                }))}
                onChange={(opt: Option) => {
                  if (!opt || !opt.value) {
                    return;
                  }
                  setValue(opt.value as EnvironmentProfile);
                }}
                value={{
                  value: value,
                  label: value,
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
              <Text>{value}</Text>
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
