import { useState } from 'react';

import { Box } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import {
  requiredEmailLike,
  requiredField,
} from 'shared/components/Validation/rules';

import {
  FormDataField,
  getSupportedEmailServices,
  SupportedEmailService,
  supportedEmailServiceLabel,
} from './types';

export function FormMixin() {
  const [sender, setSender] = useState('');
  const [fallbackRecipient, setFallbackRecipient] = useState('');
  const supportedServices = getSupportedEmailServices();
  const [service, setService] = useState<SupportedEmailService>(
    supportedServices[0]
  );

  return (
    <Box width="800px">
      <FieldInput
        width="500px"
        label="Sender"
        name={FormDataField.Sender}
        rule={requiredEmailLike}
        value={sender}
        onChange={e => setSender(e.target.value)}
        placeholder="example@goteleport.com"
        toolTipContent="Sender is the sender email"
        mb={3}
      />
      <FieldInput
        width="500px"
        label="Fallback Recipient"
        name={FormDataField.FallbackRecipient}
        rule={requiredEmailLike}
        value={fallbackRecipient}
        onChange={e => setFallbackRecipient(e.target.value)}
        placeholder="example@goteleport.com"
        toolTipContent="Fallback Recipient is the default recipient of Access Request notifications"
        mb={3}
      />
      {
        // This select must be rendered even if there's only one element, as the chosen service is
        // read from form data on submit and used in the next step. FieldSelect doesn't support
        // readonly prop which could be useful here.
      }
      <FieldSelect<Option>
        width="500px"
        label="Email Service"
        name={FormDataField.Service}
        rule={requiredField('Email Service Required')}
        value={serviceToOption(service)}
        onChange={o => setService(o.value as SupportedEmailService)}
        options={supportedServices.map(serviceToOption)}
        placeholder="Select email service"
        isSearchable={false}
        mb={3}
      />
    </Box>
  );
}

const serviceToOption = (service: SupportedEmailService): Option => ({
  value: service,
  label: supportedEmailServiceLabel(service),
});
