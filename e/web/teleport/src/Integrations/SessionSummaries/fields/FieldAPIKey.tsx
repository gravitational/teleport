import styled from 'styled-components';

import { FieldInputPassword } from 'e-teleport/Integrations/SessionSummaries/fields/FieldInput';

const APIKeyField = styled(FieldInputPassword)`
  font-family: ${p => p.theme.fonts.mono};
  font-size: 13px;
`;

interface FieldAPIKeyProps {
  autoFocus?: boolean;
  helperText?: string;
  label?: string;
}

export function FieldAPIKey(props: FieldAPIKeyProps) {
  return (
    <APIKeyField
      {...props}
      name="apiKey"
      placeholder="xxxxxx-xxxxxx"
      required={true}
    />
  );
}
