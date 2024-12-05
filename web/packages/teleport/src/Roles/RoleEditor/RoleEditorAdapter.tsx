import { Danger } from 'design/Alert';
import Flex from 'design/Flex';
import Indicator from 'design/Indicator';
import { useEffect } from 'react';
import { useAsync } from 'shared/hooks/useAsync';
import { useTheme } from 'styled-components';
import { State as ResourcesState } from 'teleport/components/useResources';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import { RoleEditor } from './RoleEditor';

/**
 * This component is responsible for converting from the `Resource`
 * representation of a role to a more accurate `RoleWithYaml` structure. The
 * conversion is asynchronous and it's performed on the server side.
 */
export function RoleEditorAdapter({
  resources,
  onSave,
  onDelete,
}: {
  resources: ResourcesState;
  onSave: (role: Partial<RoleWithYaml>) => Promise<void>;
  onDelete: () => void;
}) {
  const theme = useTheme();
  const [convertAttempt, convertToRole] = useAsync(
    async (yaml: string): Promise<RoleWithYaml | null> => {
      if (resources.status === 'creating' || !resources.item) {
        return null;
      }
      return {
        yaml,
        object: await yamlService.parse<Role>(YamlSupportedResourceKind.Role, {
          yaml,
        }),
      };
    }
  );

  const originalContent = resources.item?.content ?? '';
  useEffect(() => {
    convertToRole(originalContent);
  }, [originalContent]);

  return (
    <Flex
      flexDirection="column"
      p={4}
      borderLeft={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
      width="700px"
    >
      {convertAttempt.status === 'processing' && (
        <Flex
          flexDirection="column"
          alignItems="center"
          justifyContent="center"
          flex="1"
        >
          <Indicator />
        </Flex>
      )}
      {convertAttempt.status === 'error' && (
        <Danger>{convertAttempt.statusText}</Danger>
      )}
      {convertAttempt.status === 'success' && (
        <RoleEditor
          originalRole={convertAttempt.data}
          onCancel={resources.disregard}
          onSave={onSave}
          onDelete={onDelete}
        />
      )}
    </Flex>
  );
}
