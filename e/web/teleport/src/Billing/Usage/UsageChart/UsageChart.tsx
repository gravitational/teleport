import React, { useRef, useEffect } from 'react';
import {
  Chart,
  LinearScale,
  CategoryScale,
  BarController,
  BarElement,
  Tooltip,
} from 'chart.js';
import { Box, Text, Card } from 'design';
import theme from 'design/theme';
import { formatCents } from 'e-teleport/services/cloud';

Chart.register(LinearScale, CategoryScale, BarController, BarElement, Tooltip);

const shortMonths = [
  'JAN',
  'FEB',
  'MAR',
  'APR',
  'MAY',
  'JUN',
  'JUL',
  'AUG',
  'SEP',
  'OCT',
  'NOV',
  'DEC',
];

export default function UsageChart({ totalAmts, mt = 0 }: Props) {
  const chartContainerRef = useRef(null);

  useEffect(() => {
    // Default formatter is needed (if list is empty) to avoid
    // dividing with fractional numbers.
    let yTickFormatter = value =>
      value.toLocaleString('en-US', {
        style: 'currency',
        currency: 'USD',
      });

    if (totalAmts.length > 0) {
      yTickFormatter = value => formatCents(value);
    }

    const chart = new Chart(chartContainerRef.current, {
      type: 'bar',
      data: {
        labels: shortMonths,
        datasets: [
          {
            label: '',
            backgroundColor: theme.colors.secondary.light,
            data: totalAmts,
          },
        ],
      },
      options: {
        plugins: {
          legend: {
            display: false,
          },
          tooltip: {
            callbacks: {
              label: item => formatCents(item.parsed.y),
            },
            displayColors: false,
            padding: 8,
          },
        },
        layout: {
          padding: {
            left: 16,
            right: 32,
            top: 32,
            bottom: 16,
          },
        },
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          y: {
            ticks: {
              color: theme.colors.text.primary,
              callback: yTickFormatter,
            },
            suggestedMin: 0,
          },
          x: {
            ticks: {
              color: theme.colors.text.primary,
              padding: 8,
            },
          },
        },
      },
    });

    return () => chart.destroy();
  }, []);

  return (
    <Card overflow="hidden" mt={mt}>
      <Text typography="h4" bold px="3" py="2">
        Yearly Usage Chart
      </Text>
      <Box minHeight="215px" height="400px" bg="primary.main">
        <canvas ref={chartContainerRef} />
      </Box>
    </Card>
  );
}

type Props = {
  totalAmts: number[];
  mt?: number;
};
