import React, { useState } from 'react';
import { ButtonPrimary, Text, Box, LabelInput, Alert } from 'design';
import Validation from 'shared/components/Validation';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { requiredField } from 'shared/components/Validation/rules';
import { RequestState } from 'e-teleport/services/workflow';
import { State as RequestViewState } from '../useRequestView';

const stateOptions = [
  { value: 'APPROVED', label: 'APPROVED' },
  { value: 'DENIED', label: 'DENIED' },
];

export default function RequestReview({ attempt, submitReview, user }: Props) {
  const [state, setState] = useState<RequestState>('');
  const [reason, setReason] = useState('');

  function onSubmitReview(validator) {
    if (!validator.validate()) {
      return;
    }

    submitReview(state, reason);
  }

  // After successful submit, don't render.
  if (attempt.status === 'success') {
    return null;
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box
          border="1px solid"
          borderColor="primary.dark"
          mt={7}
          style={{ position: 'relative' }}
        >
          <Box bg="primary.dark" py={1} px={3} alignItems="center">
            <Text typography="h6" mr={3}>
              {user} - add a review
            </Text>
          </Box>
          <Box p={3} bg="primary.lighter">
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <FieldSelect
              width="200px"
              label="Request Status"
              rule={requiredField('Required')}
              placeholder="Choose status"
              value={state ? { value: state, label: state } : undefined}
              onChange={(e: Option) => setState(e.value as RequestState)}
              options={stateOptions}
            />
            <Box mb={4}>
              <LabelInput mb={1}>Message</LabelInput>
              <Box
                width="100%"
                maxWidth="500px"
                height="150px"
                as="textarea"
                p={2}
                borderRadius={2}
                placeholder="Optional message..."
                value={reason}
                onChange={e => setReason(e.target.value)}
                autoFocus
                style={{ outline: 'none', fontFamily: 'theme.font' }}
              />
            </Box>
            <ButtonPrimary
              disabled={attempt.status === 'processing'}
              onClick={() => onSubmitReview(validator)}
            >
              Submit Review
            </ButtonPrimary>
          </Box>
        </Box>
      )}
    </Validation>
  );
}

export type Props = {
  submitReview: RequestViewState['submitReview'];
  user: string;
  attempt: Attempt;
};
