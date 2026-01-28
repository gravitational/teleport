/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';

import { ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Text from 'design/Text';
import { Tabs } from 'shared/components/Editor/Tabs';
import TextEditor from 'shared/components/TextEditor/TextEditor';

import {
  IntegrationEnrollCodeType,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';

import { useTracking } from '../Shared/useTracking';
import { useGitHubK8sFlow } from './useGitHubK8sFlow';

export function CodePanel(props: {
  trackingStep: IntegrationEnrollStep;
  inProgress?: boolean;
}) {
  const { trackingStep, inProgress = false } = props;
  const [activeCodeTab, setActiveCodeTab] = useState(0);
  const [showCopyDialog, setShowCopyDialog] = useState(false);

  const { template } = useGitHubK8sFlow();

  const tracking = useTracking();

  return (
    <>
      <Tabs
        items={files}
        activeIndex={activeCodeTab}
        onSelect={setActiveCodeTab}
      />
      <TextEditor
        bg="levels.deep"
        data={[
          {
            content: makeTerraformContent(template),
            type: 'terraform',
          },
          {
            content: template.ghaWorkflow || '# Loading template...',
            type: 'yaml',
          },
        ]}
        activeIndex={activeCodeTab}
        readOnly={true}
        copyButton={!inProgress}
        downloadButton={!inProgress}
        downloadFileName={files[activeCodeTab]}
        onCopy={() => {
          tracking.codeCopy(trackingStep, trackingTypes[activeCodeTab]);
          if (inProgress) {
            setShowCopyDialog(true);
          }
        }}
        onDownload={() => {
          tracking.codeCopy(trackingStep, trackingTypes[activeCodeTab]);
        }}
      />
      <Dialog
        open={showCopyDialog}
        onClose={() => setShowCopyDialog(false)}
        dialogCss={() => ({
          maxWidth: '480px',
          width: '100%',
        })}
      >
        <DialogHeader mb={4}>
          <DialogTitle>Incomplete templates</DialogTitle>
        </DialogHeader>
        <DialogContent mb={4}>
          <Text>
            The code templates may not be complete yet. Continue to the end of
            the guide to ensure you&apos;ve added all the required details.
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonSecondary onClick={() => setShowCopyDialog(false)}>
            Ok
          </ButtonSecondary>
        </DialogFooter>
      </Dialog>
    </>
  );
}

const files = ['main.tf', 'gha-workflow.yaml'];
const trackingTypes = [
  IntegrationEnrollCodeType.Terraform,
  IntegrationEnrollCodeType.GitHubActionsYAML,
];

function makeTerraformContent(
  template: ReturnType<typeof useGitHubK8sFlow>['template']
) {
  if (template.terraform.error) {
    return `# Failed to fetch template\n# ${template.terraform.error.message}`;
  }

  if (template.terraform.data) {
    return template.terraform.data;
  }

  return '# Loading template...';
}
