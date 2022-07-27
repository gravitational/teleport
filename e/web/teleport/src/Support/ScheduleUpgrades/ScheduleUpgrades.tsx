import React from 'react';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Text from 'design/Text';
import Alert from 'design/Alert';
import Select, { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { availableUpgradeWindowStarts } from 'e-teleport/services/upgradeWindow';

import type { UpgradeWindowStart } from 'e-teleport/services/upgradeWindow';

export const makeLabel = (window: UpgradeWindowStart): string => {
  return `${window} (UTC)`;
};

const upgradeWindowOptions: Option<UpgradeWindowStart>[] =
  availableUpgradeWindowStarts.map((window: UpgradeWindowStart) => ({
    label: makeLabel(window),
    value: window,
  }));

export function ScheduleUpgrades({
  onCancel,
  onSave,
  selectedWindow,
  onSelectedWindowChange,
  attempt,
}: Props) {
  const handleChange = (selected: Option<UpgradeWindowStart>) =>
    onSelectedWindowChange(selected.value);

  return (
    <Dialog
      onCancel={onCancel}
      open={true}
      dialogCss={() => ({ maxWidth: '600px' })}
    >
      <DialogHeader>
        <DialogTitle>Select a maintenance start window</DialogTitle>
      </DialogHeader>
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      <DialogContent minWidth="500px" flex="0 0 auto">
        <Text mb={2}>
          Teleport provides different cluster maintenance windows to minimize
          downtime during upgrades, patches, etc.
        </Text>
        <Select
          onChange={handleChange}
          menuPosition="fixed"
          options={upgradeWindowOptions}
          isDisabled={attempt.status === 'processing'}
          value={{ value: selectedWindow, label: makeLabel(selectedWindow) }}
        />
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          onClick={onSave}
          disabled={attempt.status === 'processing'}
        >
          Save
        </ButtonPrimary>
        <ButtonSecondary
          onClick={onCancel}
          disabled={attempt.status === 'processing'}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

export type Props = {
  onCancel(): void;
  onSave: (window: UpgradeWindowStart) => void;
  selectedWindow: UpgradeWindowStart;
  onSelectedWindowChange: (string) => void;
  attempt: Attempt;
};
