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

import { FormEvent, ReactNode, useState } from 'react';
import { z } from 'zod';

import { ButtonPrimary, Flex, H2, Text } from 'design';
import * as Alerts from 'design/Alert';
import Validation from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAsync';

import { useLogger } from 'teleterm/ui/hooks/useLogger';

export function OfflineGateway<
  FormFieldsT extends Partial<Record<FormFieldNames, string>>,
>(props: {
  connectAttempt: Attempt<void>;
  reconnect(args: FormFieldsT): void;
  /** Gateway target displayed in the UI, for example, 'cockroachdb'. */
  targetName: string;
  /** Gateway kind displayed in the UI, for example, 'database'. */
  gatewayKind: string;
  /**
   * Each callsite is expected to pass its own formSchema that parses form data from controls passed
   * through renderFormControls. If the callsite doesn't pass any form data, it's expected to use
   * emptyFormSchema. We cannot do params.formSchema || emptyFormSchema, as that would mess with
   * type inference.
   */
  formSchema: z.ZodType<FormFieldsT>;
  /**
   * renderFormControls allows each consumer to provide its own form fields with specific HTML form
   * validation rules. The form fields are read through FormData – names on the inputs must match
   * names available through the FormFields enum.
   */
  renderFormControls?: (isProcessing: boolean) => ReactNode;
}) {
  const logger = useLogger('OfflineGateway');
  const { reconnect } = props;

  const [reconnectRequested, setReconnectRequested] = useState(false);
  const [parseError, setParseError] = useState('');

  const isProcessing = props.connectAttempt.status === 'processing';
  const statusDescription = isProcessing ? 'being set up…' : 'offline.';
  const shouldShowReconnectControls =
    props.connectAttempt.status === 'error' || reconnectRequested;

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setReconnectRequested(true);
    setParseError('');

    const formData = new FormData(event.currentTarget);
    const parseResult = props.formSchema.safeParse(
      Object.fromEntries(formData.entries())
    );

    // Explicitly compare to false to make type inference work since strictNullChecks are off.
    if (parseResult.success === false) {
      // There's no need to show validation errors in the UI, since they come from a programmer
      // error, not user inputting actually invalid data.
      logger.error('Could not parse form', parseResult.error);
      setParseError(`Could not submit form. See logs for more details.`);
      return;
    }

    reconnect(parseResult.data);
  };

  return (
    <Flex
      flexDirection="column"
      mx="auto"
      mb="auto"
      alignItems="center"
      px={4}
      css={`
        top: 11%;
        position: relative;
      `}
    >
      <H2 mb={1}>{props.targetName}</H2>
      <Text>
        The {props.gatewayKind} connection is {statusDescription}
      </Text>
      {props.connectAttempt.status === 'error' && (
        <Alerts.Danger mt={2} mb={0} details={props.connectAttempt.statusText}>
          Could not establish the connection
        </Alerts.Danger>
      )}
      {!!parseError && (
        <Alerts.Danger mt={2} mb={0} details={parseError}>
          Form validation error
        </Alerts.Danger>
      )}
      <Flex
        as="form"
        onSubmit={submitForm}
        alignItems="flex-end"
        flexWrap="wrap"
        justifyContent="space-between"
        mt={3}
        gap={2}
      >
        {shouldShowReconnectControls && (
          <>
            {props.renderFormControls && (
              // Form controls are expected to use HTML validation instead of our Validation, but
              // PortFieldInput is written in a way where it expects the context provided by
              // Validation to be present, no matter whether it's used or not.
              <Validation>{props.renderFormControls(isProcessing)}</Validation>
            )}
            <ButtonPrimary type="submit" disabled={isProcessing}>
              Reconnect
            </ButtonPrimary>
          </>
        )}
      </Flex>
    </Flex>
  );
}

export enum FormFields {
  LocalPort = 'localPort',
  TargetSubresourceName = 'targetSubresourceName',
}
type FormFieldNames = `${FormFields}`;

/**
 * emptyFormSchema is useful in situations where the callsite that uses OfflineGateway has no form
 * fields to show.
 */
export const emptyFormSchema = z.object({});
