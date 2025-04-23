/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { ButtonIcon, ButtonPrimary, Flex, H2, Link } from 'design';
import * as Alerts from 'design/Alert';
import { Cross } from 'design/Icon';
import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';

import { ShareFeedbackFormFields } from './ShareFeedbackFormFields';
import { ShareFeedbackFormValues } from './types';
import { FEEDBACK_TOO_LONG_ERROR } from './useShareFeedback';

interface ShareFeedbackProps {
  submitFeedbackAttempt: Attempt<string>;
  formValues: ShareFeedbackFormValues;

  onClose(): void;

  setFormValues(values: ShareFeedbackFormValues): void;

  submitFeedback(): Promise<[string, Error]>;
}

export function ShareFeedbackForm(props: ShareFeedbackProps) {
  const isSubmitInProgress =
    props.submitFeedbackAttempt.status === 'processing';

  return (
    <Flex bg="levels.elevated" p={3} maxWidth="370px">
      <Validation>
        {({ validator }) => (
          <Flex
            flexDirection="column"
            as="form"
            onSubmit={e => {
              e.preventDefault();
              if (validator.validate()) {
                props.submitFeedback();
              }
            }}
          >
            <Flex justifyContent="space-between" mb={2}>
              <H2>Provide your feedback</H2>
              <ButtonIcon
                type="button"
                onClick={props.onClose}
                title="Close"
                color="text.slightlyMuted"
              >
                <Cross size="medium" />
              </ButtonIcon>
            </Flex>
            <Link
              href="https://github.com/gravitational/teleport/issues/new?assignees=&labels=bug&template=bug_report.md"
              target="_blank"
            >
              Submit a Bug
            </Link>
            <Link href="https://goteleport.com/signup/" target="_blank">
              Try Teleport Cloud
            </Link>
            {props.submitFeedbackAttempt.status === 'error' && (
              <SubmissionError
                submitFeedbackAttempt={props.submitFeedbackAttempt}
              />
            )}
            {props.submitFeedbackAttempt.status === 'success' ? (
              <Alerts.Success mt={3} mb={0}>
                {props.submitFeedbackAttempt.data}
              </Alerts.Success>
            ) : (
              <>
                <ShareFeedbackFormFields
                  disabled={isSubmitInProgress}
                  formValues={props.formValues}
                  setFormValues={props.setFormValues}
                />
                <ButtonPrimary
                  disabled={isSubmitInProgress}
                  block
                  type="submit"
                  mt={4}
                >
                  Submit
                </ButtonPrimary>
              </>
            )}
          </Flex>
        )}
      </Validation>
    </Flex>
  );
}

function SubmissionError(props: { submitFeedbackAttempt: Attempt<string> }) {
  function getErrorText() {
    if (props.submitFeedbackAttempt.statusText === FEEDBACK_TOO_LONG_ERROR) {
      return (
        <span>
          That's a very long suggestion. Please let us know more in{' '}
          <Link
            href="https://github.com/gravitational/teleport/discussions"
            target="_blank"
          >
            our community
          </Link>
          .
        </span>
      );
    }

    return (
      <span>
        Unable to submit your feedback: {props.submitFeedbackAttempt.statusText}
      </span>
    );
  }

  return (
    <Alerts.Danger mt={3} mb={0}>
      {getErrorText()}
    </Alerts.Danger>
  );
}
