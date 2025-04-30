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

import { ButtonIcon, ButtonText, Flex, Text } from 'design';
import { Plus, Trash } from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';

import {
  AllowOption,
  NewJoinTokenState,
  RuleBox,
} from './UpsertJoinTokenDialog';

export const JoinTokenIAMForm = ({
  tokenState,
  onUpdateState,
  readonly,
}: {
  tokenState: NewJoinTokenState;
  onUpdateState: (newToken: NewJoinTokenState) => void;
  readonly: boolean;
}) => {
  const rules = tokenState.iam;

  function removeRule(index: number) {
    const newRules = rules.filter((_, i) => index !== i);
    const newState = {
      ...tokenState,
      iam: newRules,
    };
    onUpdateState(newState);
  }

  function setTokenRulesField(
    ruleIndex: number,
    fieldName: string,
    value: string
  ) {
    const newState = {
      ...tokenState,
      [tokenState.method.value]: tokenState[tokenState.method.value].map(
        (rule, i) => {
          if (ruleIndex !== i) {
            return rule;
          }
          return {
            ...rule,
            [fieldName]: value,
          };
        }
      ),
    };
    onUpdateState(newState);
  }

  function addNewRule() {
    const newState = {
      ...tokenState,
      iam: [...tokenState.iam, { aws_account: '' }],
    };
    onUpdateState(newState);
  }

  return (
    <>
      {rules.map((rule, index) => (
        <RuleBox
          key={index} // order does not change without updating the reference array
        >
          <Flex alignItems="center" justifyContent="space-between">
            <Text fontWeight={700} mb={2}>
              AWS Rule
            </Text>

            {rules.length > 1 && ( // at least one rule is required, so lets not allow the user to remove it
              <ButtonIcon
                data-testid="delete_rule"
                onClick={() => removeRule(index)}
              >
                <Trash size={16} color="text.muted" />
              </ButtonIcon>
            )}
          </Flex>
          <FieldInput
            label="Account ID"
            rule={requiredField('Account ID is required')}
            placeholder="AWS Account ID"
            value={rule.aws_account}
            onChange={e =>
              setTokenRulesField(index, 'aws_account', e.target.value)
            }
            readonly={readonly}
          />
          <FieldInput
            label="ARN"
            toolTipContent={`The joining nodes must match this ARN. Supports wildcards "*" and "?"`}
            placeholder="arn:aws:iam::account-id:role/*"
            value={rule.aws_arn}
            onChange={e => setTokenRulesField(index, 'aws_arn', e.target.value)}
            readonly={readonly}
          />
        </RuleBox>
      ))}
      <ButtonText onClick={addNewRule} compact>
        <Plus size={16} mr={2} />
        Add another AWS Rule
      </ButtonText>
    </>
  );
};

export const JoinTokenGCPForm = ({
  tokenState,
  onUpdateState,
  readonly,
}: {
  tokenState: NewJoinTokenState;
  onUpdateState: (newToken: NewJoinTokenState) => void;
  readonly: boolean;
}) => {
  const rules = tokenState.gcp;
  function removeRule(index: number) {
    const newRules = rules.filter((_, i) => index !== i);
    const newState = {
      ...tokenState,
      gcp: newRules,
    };
    onUpdateState(newState);
  }

  function addNewRule() {
    const newState = {
      ...tokenState,
      gcp: [
        ...tokenState.gcp,
        { project_ids: [], locations: [], service_accounts: [] },
      ],
    };
    onUpdateState(newState);
  }

  function updateRuleField(
    index: number,
    fieldName: string,
    opts: AllowOption[]
  ) {
    const newState = {
      ...tokenState,
      gcp: tokenState.gcp.map((rule, i) => {
        if (i === index) {
          return { ...rule, [fieldName]: opts };
        }
        return rule;
      }),
    };
    onUpdateState(newState);
  }

  return (
    <>
      {rules.map((rule, index) => (
        <RuleBox
          key={index} // order doesn't change without updating the referrenced array
        >
          <Flex alignItems="center" justifyContent="space-between">
            <Text fontWeight={700} mb={2}>
              GCP Rule
            </Text>

            {rules.length > 1 && ( // at least one rule is required, so lets not allow the user to remove it
              <ButtonIcon
                data-testid="delete_rule"
                onClick={() => removeRule(index)}
              >
                <Trash size={16} color="text.muted" />
              </ButtonIcon>
            )}
          </Flex>
          <FieldSelectCreatable
            placeholder="Type a Project ID"
            isMulti
            isClearable
            isSearchable
            onChange={opts =>
              updateRuleField(index, 'project_ids', opts as AllowOption[])
            }
            value={rule.project_ids}
            label="Add Project ID(s)"
            rule={requiredField('At least 1 Project ID required')}
            isDisabled={readonly}
          />
          <FieldSelectCreatable
            placeholder="us-west1, us-east1-a"
            isMulti
            isClearable
            isSearchable
            onChange={opts =>
              updateRuleField(index, 'locations', opts as AllowOption[])
            }
            value={rule.locations}
            label="Add Locations"
            helperText="Allows regions and/or zones."
            isDisabled={readonly}
          />
          <FieldSelectCreatable
            placeholder="PROJECT_compute@developer.gserviceaccount.com"
            isMulti
            isClearable
            isSearchable
            onChange={opts =>
              updateRuleField(index, 'service_accounts', opts as AllowOption[])
            }
            value={rule.service_accounts}
            label="Add Service Account Emails"
            isDisabled={readonly}
          />
        </RuleBox>
      ))}
      <ButtonText onClick={addNewRule} compact>
        <Plus size={16} mr={2} />
        Add another GCP Rule
      </ButtonText>
    </>
  );
};

export const JoinTokenOracleForm = ({
  tokenState,
  onUpdateState,
  readonly,
}: {
  tokenState: NewJoinTokenState;
  onUpdateState: (newToken: NewJoinTokenState) => void;
  readonly: boolean;
}) => {
  const rules = tokenState.oracle;
  function removeRule(index: number) {
    const newRules = rules.filter((_, i) => index !== i);
    const newState = {
      ...tokenState,
      oracle: newRules,
    };
    onUpdateState(newState);
  }

  function addNewRule() {
    const newState = {
      ...tokenState,
      oracle: [
        ...tokenState.oracle,
        { tenancy: '', parent_compartments: [], regions: [] },
      ],
    };
    onUpdateState(newState);
  }

  function updateRuleField(
    index: number,
    fieldName: string,
    opts: AllowOption[] | string
  ) {
    const newState = {
      ...tokenState,
      oracle: tokenState.oracle.map((rule, i) => {
        if (i === index) {
          return { ...rule, [fieldName]: opts };
        }
        return rule;
      }),
    };
    onUpdateState(newState);
  }

  return (
    <>
      {rules.map((rule, index) => (
        <RuleBox key={index}>
          <Flex alignItems="center" justifyContent="space-between">
            <Text fontWeight={700} mb={2}>
              Oracle Rule
            </Text>

            {rules.length > 1 && ( // at least one rule is required, so lets not allow the user to remove it
              <ButtonIcon
                data-testid="delete_rule"
                onClick={() => removeRule(index)}
              >
                <Trash size={16} color="text.muted" />
              </ButtonIcon>
            )}
          </Flex>
          <FieldInput
            label="Tenancy"
            rule={requiredField('tenancy OCID is required')}
            placeholder="ocid1.tenancy.oc1..<unique ID>"
            value={rule.tenancy}
            onChange={e => updateRuleField(index, 'tenancy', e.target.value)}
            readonly={readonly}
          />
          <FieldSelectCreatable
            placeholder="ocid1.compartment.oc1..<unique ID>"
            isMulti
            isClearable
            isSearchable
            onChange={opts =>
              updateRuleField(
                index,
                'parent_compartments',
                opts as AllowOption[]
              )
            }
            value={rule.parent_compartments}
            label="Add Compartments"
            helperText="Direct parent compartments only, no nested compartments."
            isDisabled={readonly}
          />
          <FieldSelectCreatable
            placeholder="us-ashburn-1, phx"
            isMulti
            isClearable
            isSearchable
            onChange={opts =>
              updateRuleField(index, 'regions', opts as AllowOption[])
            }
            value={rule.regions}
            label="Add Regions"
            isDisabled={readonly}
          />
        </RuleBox>
      ))}
      <ButtonText onClick={addNewRule} compact>
        <Plus size={16} mr={2} />
        Add another Oracle Rule
      </ButtonText>
    </>
  );
};
