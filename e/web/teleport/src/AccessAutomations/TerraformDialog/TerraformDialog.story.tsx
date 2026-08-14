import { http, HttpResponse } from 'msw';

import cfg from 'e-teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { TerraformDialog } from './TerraformDialog';

export default {
  title: 'TeleportE/AccessAutomations/TerraformDialog',
  component: TerraformDialog,
};

export function Success() {
  return <Component />;
}
Success.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.accessMonitoringRule.terraform, () =>
      HttpResponse.json({
        terraform: ruleTerraform,
      })
    )
  );
};

export function Loading() {
  return <Component />;
}
Loading.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.accessMonitoringRule.terraform, () => {
      return new Promise(() => {});
    })
  );
};

export function Failed() {
  return <Component />;
}
Failed.beforeEach = ({ msw }) => {
  msw.use(
    http.get(cfg.api.accessMonitoringRule.terraform, () => {
      return HttpResponse.json(
        { message: 'Failed to fetch Terraform' },
        { status: 404 }
      );
    })
  );
};

const Component = () => {
  const ctx = createTeleportContext();
  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <TerraformDialog ruleName={'example-rule'} onCancel={() => null} />
    </TeleportProviderBasic>
  );
};

const ruleTerraform = `resource "teleport_access_monitoring_rule" "test-rule" {
  version = "v1"

  metadata = {
    name = "test-rule"
  }

  spec = {
    subjects  = ["access_request"]
    condition = "some-condition"
    notification = {
      name       = "mattermost"
      recipients = ["apple", "banana", "carrot"]
    }
  }
}`;
