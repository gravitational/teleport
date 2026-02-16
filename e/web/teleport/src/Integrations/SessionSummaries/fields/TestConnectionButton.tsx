import { useCallback } from 'react';
import { useFormContext } from 'react-hook-form';

import { ButtonPrimary } from 'design/Button';
import { Check } from 'design/Icon';

import type { InferenceModelSchema } from 'e-teleport/Integrations/SessionSummaries/schema/model';
import type { TestInferenceModelRequest } from 'e-teleport/services/inference';
import { useTestInferenceModel } from 'e-teleport/services/inference/hooks';
import useStickyClusterId from 'teleport/useStickyClusterId';

interface TestConnectionButtonProps {
  onFailedMessageChange: (message: string | null) => void;
  onTestStatusChange: (status: TestStatus) => void;
  testStatus: TestStatus;
}

export enum TestStatus {
  Idle = 'idle',
  Testing = 'testing',
  Success = 'success',
  Failure = 'failure',
  NetworkError = 'network_error',
}

export function TestConnectionButton({
  onFailedMessageChange,
  onTestStatusChange,
  testStatus,
}: TestConnectionButtonProps) {
  const { clusterId } = useStickyClusterId();

  const { control, formState, getValues } =
    useFormContext<InferenceModelSchema>();

  const test = useTestInferenceModel({
    onError() {
      control._disableForm(false);

      onTestStatusChange(TestStatus.NetworkError);
      onFailedMessageChange('Network error occurred during connection test');
    },
    onMutate() {
      control._disableForm(true);

      onTestStatusChange(TestStatus.Testing);
      onFailedMessageChange(null);
    },
    onSuccess(data) {
      control._disableForm(false);

      if (data.success) {
        onTestStatusChange(TestStatus.Success);
      } else {
        onTestStatusChange(TestStatus.Failure);
        onFailedMessageChange(data.message ?? 'Connection test failed');
      }
    },
  });

  const handleTestConnection = useCallback(() => {
    test.mutate({
      clusterId,
      request: mapCredentialsFormDataToAPI(getValues()),
    });
  }, [clusterId, getValues, test]);

  const disabled =
    !formState.isValid || formState.isSubmitting || test.isPending;

  return (
    <ButtonPrimary
      disabled={disabled}
      onClick={handleTestConnection}
      pl={testStatus === TestStatus.Success ? 3 : 5}
      pr={testStatus === TestStatus.Success ? 4 : 5}
      type="button"
    >
      {testStatus === TestStatus.Success && <Check size="small" mr={2} />}
      {test.isPending ? 'Testing...' : 'Test Connection'}
    </ButtonPrimary>
  );
}

function mapCredentialsFormDataToAPI(
  data: InferenceModelSchema
): TestInferenceModelRequest {
  switch (data.accessMethod) {
    case 'bedrock':
      if (data.cloud || data.bedrockMode === 'integration') {
        return {
          bedrock: {
            integration: data.integrationName,
            modelId: data.model,
            region: data.region,
          },
        };
      }

      if (data.bedrockMode === 'inference_profile') {
        return {
          bedrock: {
            modelId: data.inferenceProfile,
            region: data.region,
          },
        };
      }

      return {
        bedrock: {
          modelId: data.model,
          region: data.region,
        },
      };

    case 'openai_api':
      return {
        openai: {
          api_key_secret_ref:
            data.apiKeyMode === 'existing' ? data.secretName : undefined,
          modelId: data.model,
        },
        secret: data.apiKeyMode === 'new' ? data.apiKey : undefined,
      };

    case 'openai_compatible':
      return {
        openai: {
          api_key_secret_ref:
            data.apiKeyMode === 'existing' ? data.secretName : undefined,
          base_url: data.apiUrl,
          modelId: data.model,
        },
        secret: data.apiKeyMode === 'new' ? data.apiKey : undefined,
      };

    case 'teleport':
      throw new Error(
        'Teleport access method is not supported in this mapping.'
      );
  }
}
