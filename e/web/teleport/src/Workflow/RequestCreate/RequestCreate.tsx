import React from 'react';
import {
  LabelInput,
  ButtonPrimary,
  ButtonSecondary,
  Alert,
  Box,
  Text,
} from 'design';
import Validation, { useRule } from 'shared/components/Validation';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';
import useTeleportE from 'e-teleport/useTeleportE';
import useRequestCreate, { State } from './useRequestCreate';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRequestCreate(ctx);
  return <RequestCreate {...state} />;
}

export function RequestCreate(props: State) {
  const {
    attempt,
    reason,
    requireReason,
    setReason,
    selectedRoles,
    setSelectedRoles,
    roles,
    createRequest,
    close,
  } = props;

  const selectOptions: Option[] = roles.map(r => ({
    value: r,
    label: r,
  }));

  function onCreateRequest(validator) {
    if (!validator.validate()) {
      return;
    }

    createRequest();
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box
          width="600px"
          p={4}
          pt={3}
          borderRadius={1}
          bg="primary.main"
          border={1}
          borderColor="primary.light"
        >
          <Text typography="h4" bold mb={3}>
            Request Role Access
          </Text>
          <Box mb={5}>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <FieldSelect
              width="300px"
              menuPosition="fixed"
              label="Roles Allowed to Request"
              rule={requiredField('At least one role is required')}
              placeholder="Click to select a role"
              isSearchable
              isMulti
              isSimpleValue
              clearable={false}
              value={selectedRoles}
              onChange={values => setSelectedRoles(values as Option[])}
              options={selectOptions}
            />
            <TextBox
              reason={reason}
              setReason={setReason}
              requireReason={requireReason}
            />
          </Box>
          <Box>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => onCreateRequest(validator)}
            >
              Send Request
            </ButtonPrimary>
            <ButtonSecondary
              disabled={attempt.status === 'processing'}
              onClick={close}
            >
              Cancel
            </ButtonSecondary>
          </Box>
        </Box>
      )}
    </Validation>
  );
}

const requireText = (value: string, requireReason: boolean) => () => {
  if (requireReason && (!value || value.trim().length === 0)) {
    return {
      valid: false,
      message: 'Reason Required',
    };
  }
  return { valid: true };
};

function TextBox({ reason, setReason, requireReason }: TextBoxProps) {
  const { valid, message } = useRule(requireText(reason, requireReason));
  const hasError = !valid;
  const labelText = hasError ? message : 'Request Reason';

  const optionalText = requireReason ? '' : ' (optional)';
  const placeholder = `Describe your request...${optionalText}`;

  return (
    <Box>
      <LabelInput hasError={hasError}>{labelText}</LabelInput>
      <Box
        as="textarea"
        height="200px"
        width="500px"
        borderRadius={2}
        p={2}
        border={hasError ? '2px solid' : '0'}
        borderColor={hasError ? 'error.dark' : 'none'}
        style={{ outline: 'none' }}
        placeholder={placeholder}
        value={reason}
        onChange={e => setReason(e.target.value)}
      />
    </Box>
  );
}

type TextBoxProps = {
  reason: State['reason'];
  setReason: State['setReason'];
  requireReason: State['requireReason'];
};
