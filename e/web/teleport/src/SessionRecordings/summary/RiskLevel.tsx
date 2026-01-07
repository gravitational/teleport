import { useTheme, type DefaultTheme } from 'styled-components';

import Flex from 'design/Flex';

import { RiskLevel as RiskLevelValue } from 'teleport/services/recordings/types';

interface RiskLevelProps {
  riskLevel: RiskLevelValue;
  inPopover?: boolean;
}

export function RiskLevel({ inPopover, riskLevel }: RiskLevelProps) {
  const theme = useTheme();
  const riskLevelColor = getRiskColor(theme, riskLevel, inPopover);

  return (
    <Flex
      inline
      alignItems="center"
      border="1px solid"
      borderColor={riskLevelColor}
      color={riskLevelColor}
      fontWeight="500"
      lineHeight={1}
      height="24px"
      px={2}
      fontSize="small"
      borderRadius="8px"
    >
      {getRiskLevelLabel(riskLevel)}
    </Flex>
  );
}

export function getRiskColor(
  theme: DefaultTheme,
  riskLevel: RiskLevelValue | undefined,
  inPopover = false
) {
  switch (riskLevel) {
    case RiskLevelValue.Low:
      return theme.colors.sessionRecording.riskLevels.low;
    case RiskLevelValue.Medium:
      return theme.colors.sessionRecording.riskLevels.medium;
    case RiskLevelValue.High:
      return theme.colors.sessionRecording.riskLevels.high;
    case RiskLevelValue.Critical:
      return theme.colors.sessionRecording.riskLevels.critical;
    default:
      const bg = inPopover
        ? theme.colors.levels.elevated
        : theme.colors.levels.sunken;
      const percent = inPopover ? 48 : 32;

      // we do not want to use rgba values here, as they do not work with the
      // gradient line between timeline items

      return `color-mix(in srgb, ${theme.colors.text.main} ${percent}%, ${bg})`;
  }
}

export function getRiskLevelLabel(riskLevel: RiskLevelValue) {
  switch (riskLevel) {
    case RiskLevelValue.Low:
      return 'Low';
    case RiskLevelValue.Medium:
      return 'Medium';
    case RiskLevelValue.High:
      return 'High';
    case RiskLevelValue.Critical:
      return 'Critical';
    default:
      return 'None';
  }
}
