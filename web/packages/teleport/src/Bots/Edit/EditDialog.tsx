/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import {
  TraitsEditor,
  TraitsOption,
} from 'shared/components/TraitsEditor/TraitsEditor';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { editBot, fetchRoles } from 'teleport/services/bot/bot';
import { FlatBot } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { formatDuration } from '../formatDuration';
import { createGetBotQueryKey, useGetBot } from '../hooks';

export function EditDialog(props: {
  botName: string;
  onCancel: () => void;
  onSuccess: (bot: FlatBot) => void;
}) {
  const { botName, onCancel, onSuccess } = props;
  const queryClient = useQueryClient();
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasReadPermission = flags.readBots;
  const hasEditPermission = flags.editBots;

  const [selectedRoles, setSelectedRoles] = useState<string[] | null>(null);
  const [selectedTraits, setSelectedTraits] = useState<TraitsOption[] | null>(
    null
  );
  const [selectedMaxSessionDuration, setSelectedMaxSessionDuration] = useState<
    string | null
  >(null);
  const { isSuccess, data, error, isLoading } = useGetBot(
    { botName },
    {
      enabled: hasReadPermission,
      staleTime: 30_000, // Keep data in the cache for 30 seconds
    }
  );

  const {
    mutate,
    error: saveError,
    isPending: isSubmitting,
  } = useMutation({
    mutationFn: editBot,
    onSuccess: newData => {
      const key = createGetBotQueryKey({ botName: newData.name });
      queryClient.setQueryData(key, newData);

      setSelectedRoles(null);
      setSelectedTraits(null);
      setSelectedMaxSessionDuration(null);

      onSuccess(newData);
    },
  });

  const handleSubmit = () => {
    const roles = selectedRoles ?? null;
    const traits =
      selectedTraits?.map(t => ({
        name: t.traitKey.value,
        values: t.traitValues.map(v => v.value),
      })) ?? null;
    const max_session_ttl =
      selectedMaxSessionDuration?.trim().replaceAll(' ', '') ?? null;

    const req = {
      roles,
      traits,
      max_session_ttl,
    };

    mutate({ botName, req });
  };

  const isDirty =
    selectedRoles !== null ||
    selectedTraits !== null ||
    selectedMaxSessionDuration !== null;

  const missingPermissions = [
    ...(hasReadPermission ? [] : ['bots.read']),
    ...(hasEditPermission ? [] : ['bots.edit']),
  ];

  return (
    <Dialog open onClose={onCancel}>
      <DialogHeader>
        <DialogTitle>Edit Bot</DialogTitle>
      </DialogHeader>
      <Validation>
        {({ validator }) => (
          <form
            onSubmit={e => {
              e.preventDefault();
              if (
                hasEditPermission &&
                isDirty &&
                !isSubmitting &&
                validator.validate()
              ) {
                handleSubmit();
              }
            }}
          >
            <DialogContent maxWidth={680}>
              {isLoading ? (
                <Box data-testid="loading" textAlign="center" m={10}>
                  <Indicator />
                </Box>
              ) : undefined}

              {error ? (
                <Alert kind="danger" details={error.message}>
                  Failed to fetch bot
                </Alert>
              ) : undefined}

              {isSuccess && data === null && (
                <Alert kind="warning">Bot {botName} does not exist</Alert>
              )}

              {missingPermissions.length ? (
                <Alert kind="info">
                  You do not have permission to edit this bot. Missing role
                  permissions: <code>{missingPermissions.join(', ')}</code>
                </Alert>
              ) : undefined}

              <div
                style={{
                  visibility:
                    hasReadPermission && isSuccess && data
                      ? 'visible'
                      : 'hidden',
                }}
              >
                <Alert kind="info" width={'100%'}>
                  Updates to a bot&apos;s identity take effect when tbot next
                  renews its certificates. By default, this happens every 20
                  minutes.
                </Alert>

                <FieldInput
                  label="Name"
                  placeholder="Name"
                  value={data?.name ?? ''}
                  readonly={true}
                  helperText={'Bot name cannot be changed'}
                />
                <FieldSelectAsync
                  menuPosition="fixed"
                  label="Roles"
                  rule={requiredField('At least one role is required')}
                  placeholder="Click to select roles"
                  isSearchable
                  isMulti
                  isClearable={false}
                  value={(selectedRoles ?? data?.roles.toSorted() ?? []).map(
                    r => ({
                      value: r,
                      label: r,
                    })
                  )}
                  onChange={(values: Option[]) =>
                    setSelectedRoles(values?.map(v => v.value) || [])
                  }
                  loadOptions={async input => {
                    const flags = ctx.getFeatureFlags();
                    const roles = await fetchRoles({ search: input, flags });
                    return roles.items.map(r => ({
                      value: r.name,
                      label: r.name,
                    }));
                  }}
                  noOptionsMessage={() => 'No roles found'}
                  elevated={true}
                />
                <Box mb={3}>
                  <TraitsEditor
                    configuredTraits={
                      selectedTraits ??
                      data?.traits
                        .toSorted((a, b) => a.name.localeCompare(b.name))
                        .map(t => ({
                          traitKey: {
                            value: t.name,
                            label: t.name,
                          },
                          traitValues: t.values.toSorted().map(v => ({
                            value: v,
                            label: v,
                          })),
                        })) ??
                      []
                    }
                    setConfiguredTraits={setSelectedTraits}
                    isLoading={false}
                    label="Traits"
                    addActionLabel="Add trait"
                    addActionSubsequentLabel="Add another trait"
                    autoFocus={false}
                  />
                </Box>
                <FieldInput
                  label="Max session duration"
                  rule={requiredField('Max session duration is required')}
                  value={
                    selectedMaxSessionDuration ??
                    formatDuration({
                      seconds:
                        data?.max_session_ttl?.seconds ??
                        TWELVE_HOURS_IN_SECONDS,
                    })
                  }
                  onChange={e => setSelectedMaxSessionDuration(e.target.value)}
                  helperText={
                    'A duration string such as 12h, 2h 45m, 43200s. Valid time units are h, m and s. Maximum is 24h'
                  }
                />
              </div>

              {saveError ? (
                <Alert kind="danger" details={saveError.message}>
                  Failed to save changes
                </Alert>
              ) : undefined}
            </DialogContent>
            <DialogFooter>
              <Flex gap={3}>
                <ButtonPrimary
                  type="submit"
                  disabled={
                    isLoading || isSubmitting || !hasEditPermission || !isDirty
                  }
                >
                  Save
                </ButtonPrimary>
                <ButtonSecondary
                  disabled={isLoading || isSubmitting}
                  onClick={onCancel}
                >
                  Cancel
                </ButtonSecondary>
              </Flex>
            </DialogFooter>
          </form>
        )}
      </Validation>
    </Dialog>
  );
}

const TWELVE_HOURS_IN_SECONDS = 43_200;
