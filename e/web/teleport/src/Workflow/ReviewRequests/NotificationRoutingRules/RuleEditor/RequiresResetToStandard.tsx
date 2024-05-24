import { OutlineInfo } from 'design/Alert/Alert';
import { ButtonSecondary, Text } from 'design';

export const RequiresResetToStandard = ({ reset }: { reset(): void }) => (
  <OutlineInfo
    mt={5}
    css={`
      flex-direction: column;
      a.external-link {
        color: ${({ theme }) => theme.colors.buttons.link.default};
      }
    `}
  >
    <Text>
      Some fields were not readable by the standard editor. To continue editing,
      go back to YAML editor or reset the affected fields to standard settings.
    </Text>
    <ButtonSecondary size="large" my={2} onClick={reset}>
      Reset to Standard Settings
    </ButtonSecondary>
  </OutlineInfo>
);
