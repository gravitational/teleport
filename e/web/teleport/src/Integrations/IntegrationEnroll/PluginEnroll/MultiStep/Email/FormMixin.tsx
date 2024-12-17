import { useState } from 'react';

import { Box } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import {
  requiredField,
  requiredEmailLike,
} from 'shared/components/Validation/rules';

import cfg from 'teleport/config';

import { FormDataField } from './types';

export function FormMixin() {
  const [sender, setSender] = useState('');
  const [fallbackRecipient, setFallbackRecipient] = useState('');
  const [service, setService] = useState<Option>({
    value: 'mailgun',
    label: 'Mailgun',
  });

  // SMTP is disabled for Cloud-Hosted Teleport
  let opts = [{ value: 'mailgun', label: 'Mailgun' }];
  if (!cfg.isCloud) {
    opts.push({ value: 'smtp', label: 'SMTP' });
  }

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
      <FieldSelect
        width="500px"
        label="Email Service"
        name={FormDataField.Service}
        rule={requiredField<Option>('Email Service Required')}
        value={service}
        onChange={o => setService(o as Option)}
        options={opts}
        placeholder="Select email service"
        toolTipContent="Email Service selects the desired email service"
        isSearchable
        mb={3}
      />
    </Box>
  );
}
