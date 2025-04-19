import { ButtonSecondary, Flex, Text } from 'design';
import { OutlineInfo } from 'design/Alert/Alert';
import { Warning } from 'design/Icon';

import { TextIcon } from 'teleport/Discover/Shared';

export const RequiresResetToStandard = ({
  reset,
  errors,
}: {
  reset(): void;
  errors: string[];
}) => (
  <OutlineInfo mt={5}>
    <Text>
      Some fields were not readable by the standard editor. To continue editing,
      go back to YAML editor or reset the affected fields to standard settings.
    </Text>
    {errors && conditionErrors(errors)}
    <ButtonSecondary size="large" my={2} onClick={reset}>
      Reset to Standard Settings
    </ButtonSecondary>
  </OutlineInfo>
);

function conditionErrors(errors: string[]) {
  return errors.map((error, index) => (
    <TextIcon key={index}>
      <Warning size="medium" mr={2} />
      <Flex alignItems="center">
        <Text>{error}</Text>
      </Flex>
    </TextIcon>
  ));
}
