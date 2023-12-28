import { IntegrationKind, MachineIdIntegration } from 'teleport/services/integrations';
import { GuidedFlow, View } from '../shared/GuidedFlow'
import { ConnectGitHub } from './ConnectGitHub'
import cfg from 'teleport/config';
import { GitHubIcon } from 'design/SVGIcon';
import { IntegrationEnrollKind } from 'teleport/services/userEvent';
import { ConfigureBot } from './ConfigureBot';
import { AddBotToWorkflow } from './AddBotToWorkflow';
import { GitHubFlowProvider } from './useGitHubFlow';

export const GitHubActionsFlow = {
  title: 'GitHub Actions',
  link: cfg.getIntegrationEnrollRoute(IntegrationKind.MachineId, MachineIdIntegration.GitHubActions),
  icon: <GitHubIcon size={80} />,
  kind: IntegrationEnrollKind.MachineIDGitHubActions,
  guided: true,
}

const views: View[] = [
  {
    name: 'Configure Bot Access',
    component: ConfigureBot,
  },
  {
    name: 'Connect GitHub',
    component: ConnectGitHub,
  },
  {
    name: 'Add Bot to GitHub',
    component: AddBotToWorkflow,
  },
]

export function EnrollMachineIdGitHub() {
  return (
    <GitHubFlowProvider>
      <GuidedFlow title="GitHub Actions and Machine ID Integration" icon={<GitHubIcon size={20} />} views={views} name={GitHubActionsFlow.title} />
    </GitHubFlowProvider>
  )
}