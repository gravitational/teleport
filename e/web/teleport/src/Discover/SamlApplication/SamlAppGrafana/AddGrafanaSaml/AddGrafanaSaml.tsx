import React, { useState, useEffect } from 'react';
import { Box } from 'design';
import { Danger } from 'design/Alert';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { AgentMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  HeaderSubtitle,
  Header,
  Mark,
  ActionButtons,
} from 'teleport/Discover/Shared';

import useTeleportE from 'e-teleport/useTeleportE';

import { EntityDescriptorInput } from '../../SamlApp';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const { prevStep, nextStep, updateAgentMeta, agentMeta } = useDiscover();

  function onSubmit(
    validator: Validator,
    name: string,
    entityDescriptor: string
  ) {
    if (!validator.validate()) {
      return;
    }
    run(() =>
      idpService.createSamlIdpServiceProvider({ name, entityDescriptor })
    );
  }

  return (
    <AddGrafanaSaml
      attempt={attempt}
      onSubmit={onSubmit}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
    />
  );
}

export function AddGrafanaSaml({
  attempt,
  onSubmit,
  agentMeta,
  updateAgentMeta,
  nextStep,
  prevStep,
}: Props) {
  const [name, setName] = useState('');
  const [entityDescriptor, setEntityDescriptor] = useState('');

  useEffect(() => {
    if (attempt.status === 'success') {
      updateAgentMeta({ ...agentMeta, resourceName: name });
      nextStep();
    }
  }, [attempt, nextStep]);

  return (
    <>
      <Header>Add SAML for Grafana To Teleport</Header>
      <HeaderSubtitle>
        Enter a name for the integration and paste your Grafana host's entity
        descriptor XML content. <br />
        You can find your entity descriptor at{' '}
        <Mark>https://{`<your-grafana-url>`}/saml/metadata</Mark>.
      </HeaderSubtitle>
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      <Box maxWidth="800px">
        <Validation>
          {({ validator }) => (
            <>
              <FieldInput
                mb={3}
                rule={requiredField('Name is required')}
                label="Name"
                autoFocus
                value={name}
                placeholder="grafana_saml"
                width="240px"
                mr="3"
                onChange={e => setName(e.target.value)}
                disabled={attempt.status === 'processing'}
              />
              <EntityDescriptorInput
                entityDescriptor={entityDescriptor}
                setEntityDescriptor={setEntityDescriptor}
              />
              <ActionButtons
                onProceed={() => onSubmit(validator, name, entityDescriptor)}
                disableProceed={attempt.status === 'processing'}
                onPrev={prevStep}
                lastStep
              />
            </>
          )}
        </Validation>
      </Box>
    </>
  );
}

export type Props = {
  attempt: Attempt;
  agentMeta: AgentMeta;
  updateAgentMeta: (meta: AgentMeta) => void;
  prevStep: () => void;
  nextStep: () => void;
  onSubmit: (
    validator: Validator,
    name: string,
    entityDescriptor: string
  ) => void;
};
