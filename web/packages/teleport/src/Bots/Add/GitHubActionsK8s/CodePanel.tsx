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

import { Tabs } from 'shared/components/Editor/Tabs';
import TextEditor from 'shared/components/TextEditor/TextEditor';

import { GHA_WORKFLOW } from './templates';
import { useGitHubK8sFlow } from './useGitHubK8sFlow';

export function CodePanel() {
  const [activeCodeTab, setActiveCodeTab] = useState(0);

  const { state, template } = useGitHubK8sFlow();

  const handleCodeTabChanged = (index: number) => {
    setActiveCodeTab(index);
  };

  return (
    <>
      <Tabs
        items={['main.tf', 'gha-workflow.yaml', 'state.json']}
        activeIndex={activeCodeTab}
        onSelect={handleCodeTabChanged}
      />
      <TextEditor
        bg="levels.deep"
        data={[
          {
            content: makeTerraformContent(template),
            type: 'terraform',
          },
          {
            content: GHA_WORKFLOW,
            type: 'yaml',
          },
          {
            content: JSON.stringify(state, undefined, 2),
            type: 'json',
          },
        ]}
        activeIndex={activeCodeTab}
        readOnly={true}
      />
    </>
  );
}

function makeTerraformContent(
  template: ReturnType<typeof useGitHubK8sFlow>['template']
) {
  if (template.error) {
    return `# Failed to fetch template\n# ${template.error.message}`;
  }

  if (template.data) {
    return template.data.terraform;
  }

  return '# Loading template...';
}
