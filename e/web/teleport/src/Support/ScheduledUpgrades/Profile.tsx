import {
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

import { availableEnvironmentProfiles } from 'e-teleport/services/cloud';
import { EnvironmentProfile } from 'e-teleport/services/cloud/cloud';
import { GetEnvironmentProfileResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import {
  EditableInput,
  ErrorTooltipIcon,
} from 'e-teleport/Support/ScheduledUpgrades/EditableInput';

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
        <Flex align="center" gap="2">
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
                <Tooltip content={mutation?.error?.message}>
                  <ErrorTooltipIcon />
                </Tooltip>
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
