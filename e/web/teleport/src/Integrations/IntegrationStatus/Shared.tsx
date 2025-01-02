import { Label, Text } from 'design';

import {
  getStatusCodeTitle,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

export const OverallStatus = ({
  statusCode,
}: {
  statusCode: IntegrationStatusCode;
}) => {
  return (
    <Label kind={getLabelKind(statusCode)}>
      <Text>{getStatusCodeTitle(statusCode)}</Text>
    </Label>
  );
};

export const getLabelKind = (statusCode: IntegrationStatusCode) => {
  switch (statusCode) {
    case IntegrationStatusCode.Unknown:
      return 'warning';
    case IntegrationStatusCode.Running:
      return 'success';
    case IntegrationStatusCode.SlackNotInChannel:
      return 'warning';
    case IntegrationStatusCode.Draft:
      return 'warning';
    default:
      // default to error kind
      return 'danger';
  }
};
