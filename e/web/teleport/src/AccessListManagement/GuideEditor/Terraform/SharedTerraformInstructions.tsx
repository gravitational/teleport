import { Stack, Text } from 'design';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { TerraformCopyButton } from 'teleport/components/TerraformCopyButton';
import { generateTshLoginCommand } from 'teleport/lib/util';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import { Terraform } from '../useGuideEditor';
import { makeTerraformContent } from './TerraformPanel';

export function SharedTerraformInstructions({
  terraform,
  isEditing = false,
}: {
  terraform: Terraform;
  isEditing?: boolean;
}) {
  const { clusterId } = useStickyClusterId();
  const ctx = useTeleport();
  const ctxUserState = ctx.storeUser.state;

  return (
    <>
      <Stack>
        <Text bold>
          1. {isEditing ? 'Update' : 'Add'} the Terraform module to your
          configuration
        </Text>
        <Text>
          Copy the Terraform module on the right and paste it into your existing
          Terraform configuration file.
        </Text>
        <TerraformCopyButton
          onClick={() => {
            copyToClipboard(makeTerraformContent(terraform));
          }}
          disabled={
            terraform.mutatePending ||
            (!terraform.config && !terraform.mutateError)
          }
        />
      </Stack>

      <Stack>
        <Text bold>2. Log in to your cluster</Text>
        <TextSelectCopyMulti
          lines={[
            {
              text: generateTshLoginCommand({
                authType: ctxUserState.authType,
                username: ctxUserState.username,
                clusterId,
              }),
            },
          ]}
        />
      </Stack>

      <Stack>
        <Text bold>3. Generate a temporary bot</Text>
        <TextSelectCopyMulti
          lines={[
            {
              text: `eval "$(tctl terraform env)"`,
            },
          ]}
        />
      </Stack>

      {!isEditing && (
        <Stack>
          <Text bold>4. Initialize Terraform</Text>
          <TextSelectCopyMulti
            lines={[
              {
                text: `terraform init`,
              },
            ]}
          />
        </Stack>
      )}

      <Stack>
        <Text bold>{isEditing ? '4' : '5'}. Plan</Text>
        <TextSelectCopyMulti
          lines={[
            {
              text: `terraform plan`,
            },
          ]}
        />
      </Stack>

      <Stack>
        <Text bold>{isEditing ? '5' : '6'}. Apply the changes</Text>
        <TextSelectCopyMulti
          lines={[
            {
              text: `terraform apply`,
            },
          ]}
        />
      </Stack>
    </>
  );
}
