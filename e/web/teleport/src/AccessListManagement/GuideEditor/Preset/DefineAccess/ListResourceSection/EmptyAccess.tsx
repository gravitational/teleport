import { Box, Flex, H1, Mark, Text } from 'design';

import { ListResourceAccessFields } from '../../role/listaccess';
import { SimpleResourceIcon } from '../../SimpleResourceIcon';

/**
 * Default empty state when user is beginning to define access
 * to a resource.
 */
export function EmptyAccess({
  resourceField,
}: {
  resourceField: ListResourceAccessFields;
}) {
  let content;
  switch (resourceField) {
    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
    case 'linux_desktop_labels':
      content = (
        <>
          To configure access, <Mark>click on labels</Mark> below or type them
          into the search bar above.
        </>
      );
      break;

    case 'github_permissions':
      content = (
        <>
          To configure access, select GitHub organizations from the dropdown
          above.
        </>
      );
      break;

    default:
      resourceField satisfies never;
  }

  return (
    <Flex
      flexDirection="column"
      alignItems="center"
      p={5}
      pt={5}
      pb={1}
      gap={2}
    >
      <Flex gap={2}>
        <SimpleResourceIcon size="extra-large" field={resourceField} />
        <H1>No Access Defined</H1>
      </Flex>
      <Box maxWidth="350px" textAlign="center">
        <Text color="text.slightlyMuted">{content}</Text>
      </Box>
    </Flex>
  );
}
