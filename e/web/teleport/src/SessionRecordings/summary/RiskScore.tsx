import { useTheme } from 'styled-components';

import Flex from 'design/Flex';
import Text from 'design/Text';

import { RiskLevel as RiskLevelValue } from 'teleport/services/recordings/types';

import { getRiskColor } from './RiskLevel';

interface RiskScoreProps {
  score: number;
  riskLevel: RiskLevelValue;
}

const STROKE_WIDTH = 4;
const CAP = STROKE_WIDTH / 2;
const ARC_WIDTH = 44;
const R = (ARC_WIDTH - STROKE_WIDTH) / 2;
const SIZE = ARC_WIDTH + CAP * 2;
const CX = SIZE / 2;
const CY = R + STROKE_WIDTH / 2 + CAP;
const SVG_HEIGHT = CY + STROKE_WIDTH / 2 + 6;

function arcPath(fraction: number) {
  const startX = CX - R;
  const startY = CY;

  if (fraction <= 0) return '';

  const endAngle = Math.PI * (1 - fraction);
  const endX = CX + R * Math.cos(endAngle);
  const endY = CY - R * Math.sin(endAngle);

  return `M ${startX} ${startY} A ${R} ${R} 0 0 1 ${endX} ${endY}`;
}

const bgArc = arcPath(1);

export function RiskScore({ score, riskLevel }: RiskScoreProps) {
  const theme = useTheme();
  const color = getRiskColor(theme, riskLevel, true);
  const clampedScore = Math.max(0, Math.min(100, score));
  const fillArc = arcPath(clampedScore / 100);

  return (
    <Flex inline alignItems="center" gap={2}>
      <Text color="text.muted">Risk Score</Text>

      <Flex
        inline
        position="relative"
        width={`${SIZE}px`}
        height={`${SVG_HEIGHT}px`}
      >
        <svg
          width={SIZE}
          height={SVG_HEIGHT}
          viewBox={`0 0 ${SIZE} ${SVG_HEIGHT}`}
          style={{ display: 'block' }}
        >
          <path
            d={bgArc}
            fill="none"
            stroke={theme.colors.spotBackground[2]}
            strokeWidth={STROKE_WIDTH}
            strokeLinecap="round"
          />
          {clampedScore > 0 && (
            <path
              d={fillArc}
              fill="none"
              stroke={color}
              strokeWidth={STROKE_WIDTH}
              strokeLinecap="round"
            />
          )}
        </svg>

        <Text
          fontSize="16px"
          fontWeight="500"
          color={color}
          style={{
            position: 'absolute',
            top: '50%',
            left: '0',
            right: 0,
            textAlign: 'center',
            transform: 'translate(0, -15%)',
            lineHeight: 1,
          }}
        >
          {score}
        </Text>
      </Flex>
    </Flex>
  );
}
