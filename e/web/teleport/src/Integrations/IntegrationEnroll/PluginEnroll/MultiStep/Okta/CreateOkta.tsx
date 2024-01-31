import React, { useState } from 'react';

import { Plugin } from 'teleport/services/integrations';

import { usePlugin } from '../../MultiStep/usePlugin';
import { SubmittablePluginForm } from '../../SubmittablePluginForm';
import { Header } from '../../MultiStep/Shared';

export function CreateOkta() {
  const { selectedPlugin, eventId, setFormData, nextStep, setInstalledPlugin } =
    usePlugin();

  const [scimToken] = useState(() => crypto.randomUUID());

  function handleSetFormData(formData: FormData) {
    // `scimToken` is the expected backend form name.
    // Do not change.
    formData.append('scimToken', scimToken);
    setFormData(formData);

    return formData;
  }

  function setStaticPluginResponse(createdPlugin: Plugin) {
    setInstalledPlugin(createdPlugin);
    nextStep();
  }

  return (
    <SubmittablePluginForm
      plugin={selectedPlugin}
      eventId={eventId}
      CustomTitle={<Header header="Features" />}
      setFormData={handleSetFormData}
      setStaticPluginResponse={setStaticPluginResponse}
    />
  );
}
