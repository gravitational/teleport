import styled from 'styled-components';

import { Box, ButtonIcon, Flex, H2 } from 'design';
import { Cross } from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import TextEditor from 'shared/components/TextEditor';

import { TerraformCopyButton } from 'teleport/components/TerraformCopyButton';

import { PanelResizer } from './PanelResizer';

export type TerraformProps = {
  data: string;
  loading: boolean;
  error: Error;
};

export function TerraformPanel({
  panelWidth,
  updatePanelWidth,
  disableResizer = false,
  terraform,
}: {
  panelWidth: number;
  updatePanelWidth(width: number): void;
  disableResizer?: boolean;
  terraform: TerraformProps;
}) {
  return (
    <CodeContainer width={panelWidth}>
      {!disableResizer && panelWidth > 0 && (
        <PanelResizer
          panelWidth={panelWidth}
          updatePanelWidth={updatePanelWidth}
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
                updatePanelWidth(0);
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
            disabled={!terraform.data}
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

export function makeTerraformContent(terraform: TerraformProps) {
  if (terraform.error) {
    return `# Failed to fetch template\n# ${terraform.error.message}`;
  }

  if (terraform.data) {
    return terraform.data;
  }

  if (terraform.loading) {
    return `# Loading template...`;
  }

  return `# Your Terraform template will appear here
# as you progress through the guide.`;
}
