import { useState } from 'react';

import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ShareFeedbackFormValues } from './types';

export const FEEDBACK_TOO_LONG_ERROR = 'FEEDBACK_TOO_LONG_ERROR';

export function useShareFeedback() {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

  const feedbackUrl = ctx.mainProcessClient.getRuntimeSettings().dev
    ? 'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod'
    : 'https://usage.teleport.dev';

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

    const response = await fetch(feedbackUrl, {
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
