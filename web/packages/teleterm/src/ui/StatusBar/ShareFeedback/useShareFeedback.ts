/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState } from 'react';

import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { staticConfig } from 'teleterm/staticConfig';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ShareFeedbackFormValues } from './types';

export const FEEDBACK_TOO_LONG_ERROR = 'FEEDBACK_TOO_LONG_ERROR';

export function useShareFeedback() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

  const [isShareFeedbackOpened, setIsShareFeedbackOpened] = useState(false);

  const [formValues, setFormValues] = useState<ShareFeedbackFormValues>(
    getFormInitialValues()
  );

  const [submitFeedbackAttempt, submitFeedback, setSubmitFeedbackAttempt] =
    useAsync(makeSubmitFeedbackRequest);

  async function makeSubmitFeedbackRequest(): Promise<string> {
    preValidateForm();

    const formData = new FormData();
    const { platform } = ctx.mainProcessClient.getRuntimeSettings();
    // The `c-` prefix is added on purpose to differentiate feedback forms sent from Connect.
    const os = `c-${platform}`;
    formData.set('OS', os);
    formData.set('email', formValues.email);
    formData.set('company', formValues.company);
    formData.set('use-case', formValues.feedback);
    formData.set('newsletter-opt-in', formValues.newsletterEnabled ? 'y' : 'n');
    formData.set('sales-opt-in', formValues.salesContactEnabled ? 'y' : 'n');

    const response = await fetch(staticConfig.feedbackAddress, {
      method: 'POST',
      body: formData,
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text);
    }
    return response.text();
  }

  function preValidateForm(): void {
    if (formValues.feedback.length > 200) {
      throw new Error(FEEDBACK_TOO_LONG_ERROR);
    }
  }

  function getEmailFromUserName(): string {
    const cluster = ctx.clustersService.findCluster(
      ctx.workspacesService.getRootClusterUri()
    );
    const userName = cluster?.loggedInUser?.name;
    if (/^\S+@\S+$/.test(userName)) {
      return userName;
    }
  }

  const hasBeenShareFeedbackOpened =
    ctx.statePersistenceService.getShareFeedbackState().hasBeenOpened;

  function openShareFeedback(): void {
    ctx.statePersistenceService.saveShareFeedbackState({ hasBeenOpened: true });
    setIsShareFeedbackOpened(true);
  }

  function closeShareFeedback(): void {
    setIsShareFeedbackOpened(false);
    clearSubmissionStatusIfSuccessful();
  }

  function clearSubmissionStatusIfSuccessful(): void {
    if (submitFeedbackAttempt.status === 'success') {
      setSubmitFeedbackAttempt(makeEmptyAttempt());
      setFormValues(getFormInitialValues());
    }
  }

  function getFormInitialValues(): ShareFeedbackFormValues {
    return {
      feedback: '',
      company: '',
      email: getEmailFromUserName() || '',
      newsletterEnabled: false,
      salesContactEnabled: false,
    };
  }

  return {
    formValues,
    submitFeedbackAttempt,
    isShareFeedbackOpened,
    hasBeenShareFeedbackOpened,
    setFormValues,
    submitFeedback,
    openShareFeedback,
    closeShareFeedback,
  };
}
