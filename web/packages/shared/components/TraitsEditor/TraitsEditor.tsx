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

import { Dispatch, SetStateAction } from 'react';
import styled from 'styled-components';

import { ButtonIcon, ButtonSecondary, Flex } from 'design';
import { buttonSizes } from 'design/ButtonIcon';
import { Cross } from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';
import { LabelContent } from 'design/LabelInput/LabelInput';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredAll, requiredField } from 'shared/components/Validation/rules';

// eslint-disable-next-line no-restricted-imports -- FIXME
import { AllUserTraits } from 'teleport/services/user';

import { ButtonWithAddIcon } from '../ButtonWithAddIcon';

/**
 * traitsPreset is a list of system defined traits in Teleport.
 * The list is used to populate traits key option.
 */
export const traitsPreset = [
  'aws_role_arns',
  'azure_identities',
  'db_names',
  'db_roles',
  'db_users',
  'gcp_service_accounts',
  'host_user_gid',
  'host_user_uid',
  'kubernetes_groups',
  'kubernetes_users',
  'logins',
  'windows_logins',
  'github_orgs',
  'mcp_tools',
  'default_relay_addr',
] as const;

export const traitDescriptions = {
  aws_role_arns: 'List of allowed AWS role ARNS',
  azure_identities: 'List of Azure identities',
  db_names: 'List of allowed database names',
  db_roles: 'List of allowed database roles',
  db_users: 'List of allowed database users',
  gcp_service_accounts: 'List of GCP service accounts',
  kubernetes_groups: 'List of allowed Kubernetes groups',
  kubernetes_users: 'List of allowed Kubernetes users',
  logins: 'List of allowed logins',
  windows_logins: 'List of allowed Windows logins',
  host_user_gid: 'The group ID to use for auto-host-users',
  host_user_uid: 'The user ID to use for auto-host-users',
  github_orgs: 'List of allowed GitHub organizations for git command proxy',
  mcp_tools: 'List of allowed MCP tools',
  default_relay_addr: 'The relay address that clients should use by default',
} as const satisfies { [key in (typeof traitsPreset)[number]]: string };

/**
 * TraitsEditor supports add, edit or remove traits functionality.
 * @param isLoading if true, it disables all the inputs in the editor.
 * @param configuredTraits holds traits configured for user in current editor.
 * @param setConfiguredTraits sets user traits in current editor.
 * @param tooltipContent sets optional tooltip content to be displayed next to the label.
 * @param label sets optional label for the editor. Default is 'User Traits'.
 */
export function TraitsEditor({
  isLoading,
  configuredTraits,
  setConfiguredTraits,
  tooltipContent,
  label = 'User Traits',
  addActionLabel = 'Add a user trait',
  addActionSubsequentLabel = 'Add another user trait',
  autoFocus = true,
}: TraitEditorProps) {
  function handleInputChange(i: InputOption | InputOptionArray) {
    const newTraits = [...configuredTraits];
    if (i.labelField === 'traitValues') {
      const traitValue: Option[] = i.option;
      if (traitValue) {
        if (traitValue[traitValue.length - 1].value.trim() === '') {
          return;
        }
        traitValue[traitValue.length - 1].label =
          traitValue[traitValue.length - 1].label.trim();
        traitValue[traitValue.length - 1].value =
          traitValue[traitValue.length - 1].value.trim();
      }
      newTraits[i.index] = {
        ...newTraits[i.index],
        [i.labelField]: traitValue ?? [],
      };
      setConfiguredTraits(newTraits);
    } else {
      const traitName: Option = i.option;
      traitName.label = traitName.label.trim();
      traitName.value = traitName.value.trim();
      newTraits[i.index] = {
        ...newTraits[i.index],
        [i.labelField]: traitName,
      };
      setConfiguredTraits(newTraits);
    }
  }

  function addNewTraitPair() {
    setConfiguredTraits([...configuredTraits, emptyTrait]);
  }

  function removeTraitPair(index: number) {
    const newTraits = [...configuredTraits];
    newTraits.splice(index, 1);
    setConfiguredTraits(newTraits);
  }

  const addLabelText =
    configuredTraits.length > 0 ? addActionSubsequentLabel : addActionLabel;

  return (
    <Fieldset>
      {label && (
        <Legend>
          <LabelContent tooltipContent={tooltipContent}>{label}</LabelContent>
        </Legend>
      )}
      <LabelTable>
        <colgroup>
          {/* Column elements (for styling purposes, see LabelTable styles) */}
          <col />
          <col />
          <col />
        </colgroup>
        {configuredTraits.length > 0 && (
          <thead>
            <tr>
              <th scope="col">
                <LabelContent required>Key</LabelContent>
              </th>
              <th scope="col">
                <LabelContent required>Value</LabelContent>
              </th>
            </tr>
          </thead>
        )}
        <tbody>
          {configuredTraits.map(({ traitKey, traitValues }, index) => {
            return (
              <tr key={index}>
                <td>
                  <FieldSelectCreatable
                    size={inputSize}
                    mb={0}
                    stylesConfig={customStyles}
                    data-testid="trait-key"
                    ariaLabel="trait-key"
                    options={traitsPreset.map(r => ({
                      value: r,
                      label: r,
                    }))}
                    placeholder="Type a trait name and press enter"
                    autoFocus={autoFocus}
                    isSearchable
                    value={traitKey}
                    rule={requiredAll(
                      requiredField('Trait key is required'),
                      requireNoDuplicateTraits(configuredTraits)
                    )}
                    onChange={e => {
                      handleInputChange({
                        option: e as Option,
                        labelField: 'traitKey',
                        index: index,
                      });
                    }}
                    createOptionPosition="last"
                    isDisabled={isLoading}
                  />
                </td>
                <td>
                  <FieldSelectCreatable
                    size={inputSize}
                    mb={0}
                    stylesConfig={customStyles}
                    data-testid="trait-values"
                    ariaLabel="trait-values"
                    placeholder="Type a trait value and press enter"
                    isMulti
                    isSearchable
                    isClearable={false}
                    value={traitValues}
                    rule={requiredField('Trait values cannot be empty')}
                    onChange={e => {
                      handleInputChange({
                        option: e as Option[],
                        labelField: 'traitValues',
                        index: index,
                      });
                    }}
                    formatCreateLabel={(i: string) =>
                      'Trait value: ' + `"${i}"`
                    }
                    isDisabled={isLoading}
                  />
                </td>
                <td>
                  <Flex
                    alignItems="center"
                    height={inputGeometry[inputSize].height}
                  >
                    <ButtonIcon
                      size={buttonIconSize}
                      title="Remove Trait"
                      aria-label="Remove Trait"
                      onClick={() => removeTraitPair(index)}
                      css={`
                        &:disabled {
                          opacity: 0.65;
                          pointer-events: none;
                        }
                      `}
                      disabled={isLoading}
                    >
                      <Cross color="text.muted" size="small" />
                    </ButtonIcon>
                  </Flex>
                </td>
              </tr>
            );
          })}
        </tbody>
      </LabelTable>
      <ButtonWithAddIcon
        Button={ButtonSecondary}
        label={addLabelText}
        onClick={addNewTraitPair}
        disabled={isLoading}
        size="small"
        pr={3}
        compact={false}
        inputAlignment
      />
    </Fieldset>
  );
}

type InputOption = {
  labelField: 'traitKey';
  option: Option;
  index: number;
};

type InputOptionArray = {
  labelField: 'traitValues';
  option: Option[];
  index: number;
};

const requireNoDuplicateTraits =
  (configuredTraits: TraitsOption[]) => (enteredTrait: Option) => () => {
    if (!enteredTrait) {
      return { valid: true };
    }
    const traitKey = configuredTraits.map(trait =>
      trait.traitKey?.value.toLowerCase()
    );
    let occurance = 0;
    traitKey.forEach(key => {
      if (key === enteredTrait.value.toLowerCase()) {
        occurance++;
      }
    });
    if (occurance > 1) {
      return { valid: false, message: 'Trait key should be unique for a user' };
    }
    return { valid: true };
  };

export const emptyTrait = {
  traitKey: null,
  traitValues: [],
};

export type TraitsOption = { traitKey: Option; traitValues: Option[] };

export type TraitEditorProps = {
  setConfiguredTraits: Dispatch<SetStateAction<TraitsOption[]>>;
  configuredTraits: TraitsOption[];
  isLoading: boolean;
  tooltipContent?: React.ReactNode;
  label?: string;
  addActionLabel?: string;
  addActionSubsequentLabel?: string;
  autoFocus?: boolean;
};

export function traitsToTraitsOption(allTraits: AllUserTraits): TraitsOption[] {
  const newTrait = [];
  for (let trait in allTraits) {
    if (!allTraits[trait]) {
      continue;
    }
    if (allTraits[trait].length === 1 && !allTraits[trait][0]) {
      continue;
    }
    if (allTraits[trait].length > 0) {
      newTrait.push({
        traitKey: { value: trait, label: trait },
        traitValues: allTraits[trait].map(t => ({
          value: t,
          label: t,
        })),
      });
    }
  }
  return newTrait;
}

const customStyles = {
  placeholder: base => ({
    ...base,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  }),
};

const inputSize = 'medium';
const buttonIconSize = 0;

const Legend = styled.legend`
  margin: 0 0 ${props => props.theme.space[1]}px 0;
  padding: 0;
  ${props => props.theme.typography.body3}
`;

const Fieldset = styled.fieldset`
  border: none;
  margin: 0;
  padding: 0;
`;

const LabelTable = styled.table`
  width: 100%;
  border-collapse: collapse;
  /*
   * Using fixed layout seems to be the only way to prevent the internal input
   * padding from somehow influencing the column width. As the padding is
   * variable (and reflects the error state), we'd rather avoid column width
   * changes while editing.
   */
  table-layout: fixed;

  & th {
    padding: 0 0 ${props => props.theme.space[1]}px 0;
  }

  col:nth-child(3) {
    /*
     * The fixed layout is good for stability, but it forces us to explicitly
     * define the width of the delete button column. Set it to the width of an
     * icon button.
     */
    width: ${buttonSizes[buttonIconSize].width};
  }

  & td {
    padding: 0;
    /* Keep the inputs top-aligned to support error messages */
    vertical-align: top;
    padding-bottom: ${props => props.theme.space[2]}px;

    &:nth-child(1),
    &:nth-child(2) {
      padding-right: ${props => props.theme.space[2]}px;
    }
  }
`;
