/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useEffect, useState } from 'react';
import { useTheme } from 'styled-components';

import { LabelInput, Link, Mark } from 'design';
import { OutlineInfo } from 'design/Alert/Alert';
import { Cross } from 'design/Icon';
import { P } from 'design/Text/Text';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';

import { AWS_TAG_INFO_LINK } from 'teleport/Discover/Shared/const';
import { Option } from 'teleport/Discover/Shared/SelectCreatable';
import { styles } from 'teleport/Discover/Shared/SelectCreatable/SelectCreatable';
import {
  SetupAccessWrapper,
  useUserTraits,
} from 'teleport/Discover/Shared/SetupAccess';
import { IAM_ROLE_ARN_REGEX } from 'teleport/services/integrations/aws';

export function SetupAccess() {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    ...restOfProps
  } = useUserTraits();

  const theme = useTheme();

  const [selectedArns, setSelectedArns] = useState<Option[]>([]);
  const [inputValue, setInputValue] = useState('');

  useEffect(() => {
    if (restOfProps.attempt.status === 'success') {
      setSelectedArns(initSelectedOptions('awsRoleArns'));
    }
  }, [restOfProps.attempt.status]);

  function handleKeyDown(event: React.KeyboardEvent, validator: Validator) {
    if (!inputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        if (!validator.validate()) return;
        setSelectedArns(prevArns => [
          ...prevArns,
          { value: inputValue, label: inputValue },
        ]);
        setInputValue('');
        event.preventDefault();
    }
  }

  function handleOnProceed(validator: Validator) {
    if (!validator.validate()) return;
    onProceed({ awsRoleArns: selectedArns });
  }

  const canAddTraits = !restOfProps.isSsoUser && restOfProps.canEditUser;
  const headerSubtitle = 'Allow access to AWS Management Console.';
  const hasTraits = selectedArns.length > 0;

  const preContent = (
    <OutlineInfo mt={-3} mb={3} linkColor="buttons.link.default">
      <P>
        Only{' '}
        <Link target="_blank" href={AWS_TAG_INFO_LINK}>
          IAM roles with tag
        </Link>{' '}
        key <Mark>teleport.dev/integration</Mark> and value <Mark>true</Mark>{' '}
        are allowed to be used by the integration.
      </P>
    </OutlineInfo>
  );

  return (
    <Validation>
      {({ validator }) => (
        <SetupAccessWrapper
          {...restOfProps}
          headerSubtitle={headerSubtitle}
          traitKind="ARN"
          traitDescription=""
          hasTraits={hasTraits}
          onProceed={() => handleOnProceed(validator)}
          preContent={preContent}
          onPrev={null}
        >
          <FieldSelectCreatable
            css={`
              ${LabelInput} {
                font-size: ${p => p.theme.fontSizes[2]}px;
                padding-bottom: ${p => p.theme.space[1]}px;
              }
            `}
            mb={1}
            components={{
              DropdownIndicator: null,
              CrossIcon: () => <Cross />,
            }}
            stylesConfig={styles(theme)}
            rule={validArns}
            label="Copy and paste IAM role ARNs that you want to assume when accessing the
              AWS console"
            inputValue={inputValue}
            onInputChange={setInputValue}
            onKeyDown={(e: React.KeyboardEvent) => handleKeyDown(e, validator)}
            isMulti
            isSearchable
            isClearable={selectedArns.some(v => !v.isFixed)}
            placeholder="Copy and paste ARNs"
            value={selectedArns}
            isDisabled={!canAddTraits}
            onChange={(value: Option[], action) => {
              if (action.action === 'clear') {
                setSelectedArns(getFixedOptions('awsRoleArns'));
              } else {
                setSelectedArns(value || []);
              }
            }}
            options={getSelectableOptions('awsRoleArns')}
            autoFocus
            toolTipContent={
              <>
                ARN is found in the format{' '}
                <Mark>{`arn:aws:iam::<AWS_ACCOUNT_ID>:role/<NAME_OF_ROLE>`}</Mark>
              </>
            }
          />
        </SetupAccessWrapper>
      )}
    </Validation>
  );
}

export const validArns = (createdArns: Option[]) => () => {
  if (!createdArns) {
    return {
      valid: true,
    };
  }

  const badFilters = createdArns.filter(o => {
    // Ignore statically (role) configured arns since
    // user cannot fix it on this screen.
    return !o.isFixed && !IAM_ROLE_ARN_REGEX.test(o.value);
  });

  if (badFilters.length > 0) {
    return {
      valid: false,
      message: `The following ARNs have invalid format: ${badFilters
        .map(f => f.value)
        .join(', ')}`,
    };
  }

  return { valid: true };
};
