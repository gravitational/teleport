import { useQuery } from '@tanstack/react-query';

import { Alert } from 'design/Alert';
import { ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Flex from 'design/Flex';
import TextEditor from 'shared/components/TextEditor';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import useStickyClusterId from 'teleport/useStickyClusterId';

export function TerraformDialog({
  ruleName,
  onCancel,
}: {
  ruleName: string;
  onCancel: () => void;
}) {
  const { clusterId } = useStickyClusterId();
  const {
    data = '',
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: ['access_monitoring_rule', 'terraform', ruleName],
    queryFn: ({ signal }) =>
      accessMonitoringRuleService.fetchAccessMonitoringRuleTerraform(
        {
          clusterId,
          name: ruleName,
        },
        signal
      ),
    enabled: Boolean(ruleName),
    staleTime: 30_000, // Cached terraform is valid for 30 seconds
  });

  return (
    <Dialog
      dialogCss={() => ({
        height: '80%',
        width: '80%',
        maxHeight: '1000px',
        maxWidth: '1400px',
      })}
      onClose={() => onCancel?.()}
      open={true}
    >
      <DialogHeader>
        <Flex data-testid="terraform" flex="1" flexDirection="column">
          <DialogTitle>{ruleName}</DialogTitle>
          {isError && (
            <Alert mt={4} mb={0} kind="danger" details={error.message}>
              Failed to fetch Terraform resource
            </Alert>
          )}
        </Flex>
      </DialogHeader>
      <DialogContent>
        <TextEditor
          readOnly={true}
          data={[
            {
              content: isLoading ? '# Fetching Terraform…' : data,
              type: 'terraform',
            },
          ]}
          copyButton={true}
          downloadButton={true}
          downloadFileName={`access_monitoring_rule_${ruleName}.tf`}
        />
      </DialogContent>
      <DialogFooter>
        <Flex justifyContent="flex-end">
          <ButtonSecondary width="50%" onClick={onCancel}>
            Close
          </ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}
