/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React, { memo, useCallback, useEffect } from 'react';
import { useTheme } from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import Text from 'design/Text';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { precomputed, requiredAll } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { LabelsInput } from 'teleport/components/LabelsInput';
import { ApiError } from 'teleport/services/api/parseError';
import useTeleport from 'teleport/useTeleport';

import { SectionPadding, SectionPropsWithDispatch } from './sections';
import { MetadataModel, roleVersionOptions } from './standardmodel';
import { ActionType } from './useStandardModel';
import { MetadataValidationResult } from './validation';

export const MetadataSection = memo(
  ({
    value,
    isProcessing,
    validation,
    isEditing,
    dispatch,
  }: SectionPropsWithDispatch<MetadataModel, MetadataValidationResult> & {
    isEditing: boolean;
  }) => {
    const theme = useTheme();
    const { resourceService } = useTeleport();

    // Verifies whether a role already exists with this name. This will be used
    // to show a validation error if necessary.
    const [, checkForNameCollisionNow] = useAsync(
      useCallback(
        async (name: string) => {
          if (isEditing || name === '') {
            return;
          }
          try {
            await resourceService.fetchRole(name);
            // `fetchRole` will throw an error if HTTP/404 is returned,
            // indicating that there's no such role. If we end up here, it
            // means that there is already a role with this name, so we need to
            // report it back to the model.
            //
            // Compatibility note: if we hit a proxy that doesn't yet support
            // this endpoint, we just get a 404 anyway, which simply means the
            // validation won't work in this case, but it won't be a
            // catastrophic failure; the user will just get their role rejected
            // by the server and will see a server error instead.
            dispatch({
              type: ActionType.SetRoleNameCollision,
              payload: true,
            });
          } catch (e) {
            // If there's no such role, don't do anything. That's expected.
            // Otherwise, print the exception, since it would be swallowed by
            // `useAsync` anyway, and we don't use the returned attempt object.
            if (!(e instanceof ApiError && e.response.status === 404)) {
              console.error(e);
            }
          }
        },
        [resourceService, dispatch]
      )
    );

    // Getting callback caching right is especially important here, as we want
    // always to use the correct version of `checkForNameCollisionNow`, but we
    // also don't want to call `debounce` on every render, as it would defeat
    // the purpose of debouncing altogether.
    const checkForNameCollision = useCallback(
      debounce(checkForNameCollisionNow, 500),
      [checkForNameCollisionNow]
    );

    useEffect(() => {
      // Check if the default name is available right after the component is
      // rendered for the first time.
      checkForNameCollision(value.name);
    }, []);

    function handleChange(newValue: MetadataModel) {
      dispatch({ type: ActionType.SetMetadata, payload: newValue });
    }

    function handleNameChange(e: React.ChangeEvent<HTMLInputElement>) {
      const name = e.target.value;
      handleChange({ ...value, name, nameCollision: false });
      checkForNameCollision(name);
    }

    return (
      <Flex flexDirection="column" gap={3}>
        <SectionPadding>Basic information about this role</SectionPadding>
        <Box
          border={1}
          borderColor={theme.colors.interactive.tonal.neutral[0]}
          borderRadius={3}
          p={3}
        >
          <FieldInput
            label="Role Name"
            required
            placeholder="Enter Role Name"
            value={value.name}
            disabled={isProcessing}
            readonly={isEditing}
            rule={requiredAll(
              precomputed(validation.fields.name),
              precomputed(validation.fields.nameCollision)
            )}
            onChange={handleNameChange}
          />
          <FieldTextArea
            label="Description"
            placeholder="Enter Role Description"
            value={value.description || ''}
            disabled={isProcessing}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              handleChange({ ...value, description: e.target.value })
            }
          />
          <Box mb={3}>
            <Text typography="body3" mb={1}>
              Labels
            </Text>
            <LabelsInput
              disableBtns={isProcessing}
              labels={value.labels}
              setLabels={labels => handleChange({ ...value, labels })}
              rule={precomputed(validation.fields.labels)}
            />
          </Box>
          <FieldSelect
            label="Version"
            isDisabled={isProcessing}
            options={roleVersionOptions}
            value={value.version}
            onChange={version => handleChange({ ...value, version })}
            mb={0}
            menuPosition="fixed"
          />
        </Box>
      </Flex>
    );
  }
);
