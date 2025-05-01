import { Label, Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';

import { IntegrationLike } from 'teleport/Integrations/IntegrationList';
import {
  getStatusCodeDescription,
  getStatusCodeTitle,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

export const OverallStatus = ({
  statusCode,
  status = undefined,
}: {
  statusCode: IntegrationStatusCode;
  status?: IntegrationLike['status'];
}) => {
  const statusDescription = status?.errorMessage
    ? getStatusCodeDescription(statusCode, status.errorMessage)
    : undefined;

  return (
    <HoverTooltip tipContent={statusDescription}>
      <Label kind={getLabelKind(statusCode)}>
        <Text>{getStatusCodeTitle(statusCode)}</Text>
      </Label>
    </HoverTooltip>
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
    case IntegrationStatusCode.OktaConfigError:
    default:
      // default to error kind
      return 'danger';
  }
};
