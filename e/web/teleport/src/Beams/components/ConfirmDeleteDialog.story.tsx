import type { Meta, StoryObj } from '@storybook/react-vite';

import { Beam } from 'e-teleport/services/beams/types';

import { ConfirmDeleteDialog } from './ConfirmDeleteDialog';

const ALIAS_PREFIXES = [
  'far-flight',
  'idle-stream',
  'noisy-star',
  'bright-coil',
  'gentle-spark',
  'curious-flight',
  'ideal-mesh',
  'still-asteroid',
  'kind-starburst',
  'silver-pulse',
  'mellow-tide',
  'rapid-glow',
  'lively-spire',
  'soft-arc',
  'distant-coil',
  'cosmic-author',
  'silent-light',
  'solid-flux',
  'patient-screen',
  'kind-stream',
  'dawn-shadow',
  'electric-mesa',
  'velvet-river',
  'stellar-data',
];

function makeBeams(count: number): Beam[] {
  return Array.from({ length: count }, (_, i) => ({
    name: `00000000-0000-0000-0000-${String(i).padStart(12, '0')}`,
    alias:
      ALIAS_PREFIXES[i % ALIAS_PREFIXES.length] +
      (i >= ALIAS_PREFIXES.length ? `-${i}` : ''),
    user: 'llama',
    expires: '2027-01-01T00:00:00Z',
    node_id: 'node-1',
    app_name: '',
    egress_mode: 'unrestricted',
    allowed_domains: [],
    compute_status: 'provision_complete',
  }));
}

const meta = {
  title: 'TeleportE/Beams/ConfirmDeleteDialog',
  component: ConfirmDeleteDialog,
  args: {
    isPending: false,
    onClose: () => {},
    onConfirm: () => {},
  },
} satisfies Meta<typeof ConfirmDeleteDialog>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Single: Story = {
  args: { beams: makeBeams(1) },
};

export const Double: Story = {
  args: { beams: makeBeams(2) },
};

export const Multi: Story = {
  args: { beams: makeBeams(5) },
};

export const AtDisplayCap: Story = {
  args: { beams: makeBeams(20) },
};

// With 35 beams, the first 20 names are shown, the remainder collapsed into "and N more".
export const OverDisplayCap: Story = {
  args: { beams: makeBeams(35) },
};

export const Pending: Story = {
  args: { beams: makeBeams(3), isPending: true },
};
