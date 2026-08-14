import styled from 'styled-components';

import { Box, ButtonIcon, Flex, H2 } from 'design';
import { Cross } from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import TextEditor from 'shared/components/TextEditor';

import { TerraformCopyButton } from 'teleport/components/TerraformCopyButton';

import { Terraform } from '../useGuideEditor';
import { PanelResizer } from './PanelResizer';

export function TerraformPanel({
  disableResizer = false,
  terraform,
}: {
  disableResizer?: boolean;
  terraform: Terraform;
}) {
  const { sidePanel: panelWidth, updateSidePanel } = terraform;
  return (
    <CodeContainer width={panelWidth}>
      {!disableResizer && panelWidth > 0 && (
        <PanelResizer
          panelWidth={panelWidth}
          updatePanelWidth={updateSidePanel}
        />
      )}
      <Flex flexDirection={'column'} width="100%">
        <Flex
          gap={2}
          alignItems="center"
          justifyContent="space-between"
          px={3}
          py={2}
        >
          <H2>Terraform</H2>
          {!disableResizer && (
            <ButtonIcon
              onClick={() => {
                updateSidePanel(0);
              }}
            >
              <Cross size="small" />
            </ButtonIcon>
          )}
        </Flex>
        <TextEditor
          bg="levels.deep"
          data={[
            {
              content: makeTerraformContent(terraform),
              type: 'terraform',
            },
          ]}
          readOnly={true}
        />
        <Box p={3}>
          <TerraformCopyButton
            onClick={() => {
              copyToClipboard(makeTerraformContent(terraform));
            }}
            disabled={
              terraform.mutatePending ||
              (!terraform.config && !terraform.mutateError)
            }
          />
        </Box>
      </Flex>
    </CodeContainer>
  );
}

const CodeContainer = styled(Flex)`
  overflow: hidden;
  border-left: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[1]};
  background: ${p => p.theme.colors.levels.surface};
  position: relative;
`;

export function makeTerraformContent(terraform: Terraform) {
  if (terraform.mutateError) {
    return `# Failed to fetch template\n# ${terraform.mutateError.message}`;
  }

  if (terraform.config) {
    return terraform.config;
  }

  if (terraform.mutatePending) {
    return `# Loading template...`;
  }

  return `# Your Terraform template will appear here
# as you progress through the guide.`;
}
