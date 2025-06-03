import styled, { useTheme } from 'styled-components';

import { Box, Flex, H2, H3, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { UPGRADE_POLICY_URL } from 'teleport/services/sales';

import { isCalibrationPeriod } from './SummaryPage';
import { ProductUsage } from './types';
import { UpdatedAtDisplay } from './UpdatedAtDisplay';
import { UsageBar } from './UsageBar';

// MWI_PER_MAU is how many free MWI customers get for each MAU they aquire.
// This value is used to tell if a customer has bought additional MWI and hence is
// in the new price model, or not.
// TODO(mcbattirola): This is temporary and will be removed in fall 2025.
const MWI_PER_MAU = 0.5;

export interface CycleProps {
  summary: UsageSummary;
  hasIdentityGovernance: boolean;
  hasIdentitySecurity: boolean;
}

export const Cycle = ({
  summary: {
    cloud,
    cycleEnd,
    cycleEndFormatted,
    cycleStart,
    cycleStartFormatted,
    mau,
    tpr,
    mwi,
    igmau,
    hasCloudAnonymizationKey,
    salesforceIdUpdatedAt,
    usageUpdatedAt,
    usageUpdatedAtFormatted,
  },
  hasIdentityGovernance,
  hasIdentitySecurity,
}: CycleProps) => {
  const theme = useTheme();
  const calibrationPeriod = isCalibrationPeriod(
    cloud,
    cycleStart,
    cycleEnd,
    hasCloudAnonymizationKey,
    salesforceIdUpdatedAt
  );

  // hasExtraMwi is used to show or hide MWI's blurb, which contains additional info
  // that only customers in the old price model should see.
  // Ideally, this information should come from the Cloud backend, but since this is
  // temporary and all products use the same MWI per MAU (0.5), we hardcoded it here.
  // TODO(mcbattirola): remove this and MWI blurb completely on v19.
  const hasExtraMwi = mwi.maximum > Math.ceil(MWI_PER_MAU * mau.maximum);

  const productUsages: ProductUsage[] = [
    {
      name: 'Zero Trust Access',
      info: 'A secure, on-demand, least-privileged access to infrastructure using cryptographic identity and Zero Trust principles.',
      enabled: true, // always enabled
      usages: [
        {
          name: 'Monthly Active Users (MAU)',
          total: mau.cycleCount,
          percentage: calibrationPeriod
            ? 100
            : ~~Math.round((mau.cycleCount / mau.maximum) * 100),
          percentageMax: mau.maximum,
          hardMax: mau.maximum,
        },
        {
          name: 'Teleport Protected Resources (TPR)',
          total: tpr.cycleCount,
          percentage: calibrationPeriod
            ? 100
            : ~~Math.round((tpr.cycleCount / tpr.maximum) * 100),
          percentageMax: tpr.maximum,
          hardMax: tpr.maximum,
        },
      ],
    },
    {
      name: 'Machine and Workload Identities',
      info: 'Improve infrastructure resiliency by securing access to systems  and data between machines & workloads.',
      enabled: true, // always enabled
      usages: [
        {
          name: 'MWI',
          total: mwi.cycleCount,
          percentage: calibrationPeriod
            ? 100
            : ~~Math.round((mwi.cycleCount / mwi.maximum) * 100),
          percentageMax: mwi.maximum,
          hardMax: mwi.maximum,
        },
      ],
      blurb: hasExtraMwi
        ? null
        : 'MWIs were previously counted as TPRs, but are now part of a new product. Billing will remain consistent with your current contract.',
    },
    {
      name: 'Identity Governance',
      info: 'Harden your infrastructure with identity governance and security.',
      enabled: hasIdentityGovernance,
      ctaUrl: UPGRADE_POLICY_URL,
      usages: [
        {
          name: 'Monthly Active Users (MAU)',
          total: igmau.cycleCount,
          percentage: calibrationPeriod
            ? 100
            : ~~Math.round((igmau.cycleCount / igmau.maximum) * 100),
          percentageMax: igmau.maximum,
          hardMax: igmau.maximum,
        },
      ],
    },
    {
      name: 'Identity Security',
      info: 'Secure identities and access policies across all of your infrastructure. Eliminate shadow access and blind spots.',
      enabled: hasIdentitySecurity,
      ctaUrl: '',
      usages: [
        {
          name: 'Teleport Protected Resources (TPR)',
          total: tpr.cycleCount,
          percentage: calibrationPeriod
            ? 100
            : ~~Math.round((tpr.cycleCount / tpr.maximum) * 100),
          percentageMax: tpr.maximum,
          hardMax: tpr.maximum,
        },
      ],
    },
  ];

  return (
    <Box>
      <Flex gap="3" alignItems="center">
        <H2>
          Current Billing Cycle: {cycleStartFormatted} - {cycleEndFormatted}
        </H2>
        {calibrationPeriod && (
          <Flex
            gap="1"
            alignItems="center"
            bg="interactive.tonal.neutral.0"
            borderRadius="35px"
            px="2"
            py="1"
          >
            <IconTooltip>
              A change to your account requires a calibration period in order to
              accurately count Active Users and Teleport Protected Resources.
              This should resolve itself with the start of your next billing
              cycle.
            </IconTooltip>
            <CalibrationText>Calibration In Progress</CalibrationText>
          </Flex>
        )}
      </Flex>
      <Text color={theme.colors.text.slightlyMuted} mt="2">
        Monthly usage will reset at the end of this cycle
      </Text>
      <Flex gap="3" flexWrap="wrap" my="3">
        {productUsages.map(p => (
          <CyclesContainer
            key={p.name}
            data-testid={p.name}
            enabled={p.enabled}
          >
            <H3>{p.name}</H3>
            <Text color="text.slightlyMuted" mt="2" fontWeight={300}>
              {p.info}
            </Text>
            <Box mt="4">
              <UsageBar productUsage={p} calibrating={calibrationPeriod} />
            </Box>
            {p.blurb && (
              <Text color="text.muted" mt="4" fontWeight={400}>
                {p.blurb}
              </Text>
            )}
          </CyclesContainer>
        ))}
      </Flex>

      <UpdatedAtDisplay
        theme={theme}
        usageUpdatedAt={usageUpdatedAt}
        usageUpdatedAtFormatted={usageUpdatedAtFormatted}
      />
    </Box>
  );
};

const CyclesContainer = styled(Flex)<{ enabled?: boolean }>`
  flex: 1 1 33%;
  min-width: 420px;
  background-color: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  padding: ${({ theme }) => theme.space[4]}px;
  flex-direction: column;
  justify-content: ${({ enabled }) => (enabled ? 'normal' : 'space-between')};
`;

const CalibrationText = styled(Text)`
  display: none;
  font-size: ${p => p.theme.fontSizes[1]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.medium}) {
    display: inline;
  }
`;
