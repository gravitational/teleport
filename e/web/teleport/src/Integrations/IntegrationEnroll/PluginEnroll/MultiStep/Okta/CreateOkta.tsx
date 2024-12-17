import { useState } from 'react';

import { usePlugin } from '../../MultiStep/usePlugin';
import { SubmittablePluginForm } from '../../SubmittablePluginForm';
import { Header } from '../../MultiStep/Shared';

import { FormDataField } from './types';

export function CreateOkta() {
  const { selectedPlugin, eventId, setFormData, nextStep } = usePlugin();

  const [scimToken] = useState(() => crypto.randomUUID());

  function handleSetFormData(formData: FormData) {
    let orgUrl = formData.get(FormDataField.OrgUrl).toString();

    if (!orgUrl.startsWith('http://') && !orgUrl.startsWith('https://')) {
      orgUrl = `https://${orgUrl}`;
    }
    formData.set(FormDataField.OrgUrl, orgUrl);

    // `scimToken` is the expected backend form name.
    // Do not change.
    formData.set(FormDataField.ScimToken, scimToken);

    setFormData(formData);
    nextStep();
  }

  return (
    <SubmittablePluginForm
      plugin={selectedPlugin}
      eventId={eventId}
      CustomTitle={<Header header="Features" />}
      setFormData={handleSetFormData}
    />
  );
}
