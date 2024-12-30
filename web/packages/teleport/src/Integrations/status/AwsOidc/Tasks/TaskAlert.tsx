import { Alert } from 'design';
import { ArrowForward, BellRinging } from 'design/Icon';
import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';
import { useHistory } from 'react-router';

type TaskAlertProps = {
  name: string;
  total: number;
  kind?: IntegrationKind;
};

export function TaskAlert({
  name,
  total,
  kind = IntegrationKind.AwsOidc,
}: TaskAlertProps) {
  const history = useHistory();

  return (
    <Alert
      kind="warning"
      icon={BellRinging}
      primaryAction={{
        content: (
          <>
            Resolve Now
            <ArrowForward size={18} ml={2} />
          </>
        ),
        onClick: () => history.push(cfg.getIntegrationTasksRoute(kind, name)),
      }}
    >
      {total} Pending Tasks
    </Alert>
  );
}
