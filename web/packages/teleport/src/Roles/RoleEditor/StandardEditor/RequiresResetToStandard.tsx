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

import { useState } from 'react';

import {
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Mark,
  P2,
  Text,
} from 'design';
import { Info } from 'design/Alert/Alert';
import Dialog, { DialogContent, DialogHeader } from 'design/Dialog';

import {
  ConversionError,
  ConversionErrorGroup,
  ConversionErrorType,
} from './errors';

/**
 * Informs the user that the role they're trying to edit is not supported by
 * the standard editor. Provides a dialog that explains the details of what's
 * not supported and lets the user commit the proposed changes.
 */
export const RequiresResetToStandard = ({
  conversionErrors,
  onReset,
}: {
  conversionErrors: ConversionErrorGroup[];
  /**
   * Called if the user decides to reset the role to be compatible with the
   * standard model.
   */
  onReset(): void;
}) => {
  const [dialogOpen, setDialogOpen] = useState(false);
  const handleReviewClick = () => {
    setDialogOpen(true);
  };
  const handleCancelClick = () => {
    setDialogOpen(false);
  };
  return (
    <Info alignItems="flex-start">
      <Text>
        This role is too complex to be edited in the standard editor. To
        continue editing, go back to YAML editor or reset the affected fields to
        standard settings.
      </Text>
      <ButtonSecondary size="large" my={2} onClick={handleReviewClick}>
        Review and Reset to Standard Settings
      </ButtonSecondary>
      <Dialog open={dialogOpen}>
        <DialogHeader mb={4}>
          <H2>Review and Reset to Standard Settings</H2>
        </DialogHeader>
        <DialogContent mb={3}>
          <P2>
            To support editing this role in the standard editor, the following
            fields will be altered. Please review the list carefully before
            continuing.
          </P2>

          {conversionErrors.map(group => (
            // group.type is unique per contract of
            // `groupAndSortConversionErrors()`.
            <ErrorGroupSection key={group.type} group={group} />
          ))}
        </DialogContent>
        <Flex gap={3}>
          <ButtonPrimary block size="large" onClick={onReset}>
            Reset to Standard Settings
          </ButtonPrimary>
          <ButtonSecondary block size="large" onClick={handleCancelClick}>
            Cancel
          </ButtonSecondary>
        </Flex>
      </Dialog>
    </Info>
  );
};

const ErrorGroupSection = ({
  group: { type, errors },
}: {
  group: ConversionErrorGroup;
}) => {
  return (
    <>
      <ErrorGroupSectionHeader type={type} />
      <ul>
        {errors.map((error, i) => (
          <li key={`${i}:${error.path}`}>
            <ErrorMessage error={error} />
          </li>
        ))}
      </ul>
    </>
  );
};

const ErrorGroupSectionHeader = ({ type }: { type: ConversionErrorType }) => {
  switch (type) {
    case ConversionErrorType.UnsupportedField:
      return <P2>The following fields are unsupported and will be deleted:</P2>;
    case ConversionErrorType.UnsupportedValue:
      return (
        <P2>
          The following fields have unsupported values and will be deleted:
        </P2>
      );
    case ConversionErrorType.UnsupportedValueWithReplacement:
      return (
        <P2>
          The following fields have unsupported values and will be replaced:
        </P2>
      );
    case ConversionErrorType.UnsupportedChange:
      return (
        <P2>
          Modifying the following fields is not supported at all, and they will
          be reset to their original value:
        </P2>
      );
    default:
      (type) satisfies never;
  }
};

const ErrorMessage = ({ error }: { error: ConversionError }) => {
  const { type, path } = error;
  switch (type) {
    case ConversionErrorType.UnsupportedField:
    case ConversionErrorType.UnsupportedValue:
    case ConversionErrorType.UnsupportedChange:
      return <Mark>{path}</Mark>;
    case ConversionErrorType.UnsupportedValueWithReplacement:
      return (
        <>
          <Mark>{path}</Mark>: will be replaced with{' '}
          <Mark>{error.replacement}</Mark>
        </>
      );
    default:
      (type) satisfies never;
  }
};
