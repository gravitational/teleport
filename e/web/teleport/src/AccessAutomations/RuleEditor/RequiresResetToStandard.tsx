import { ButtonSecondary, Text } from 'design';
import { OutlineInfo } from 'design/Alert/Alert';

export const RequiresResetToStandard = ({ reset }: { reset(): void }) => (
  <OutlineInfo mt={5}>
    <Text>
      Some fields were not readable by the standard editor. To continue editing,
      go back to YAML editor or reset fields to standard settings.
    </Text>
    <ButtonSecondary size="large" my={2} onClick={reset}>
      Reset to Standard Settings
    </ButtonSecondary>
  </OutlineInfo>
);
