import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Text from 'design/Text';
import { Alert } from 'design/Alert';
import Select, { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { availableUpgradeWindowStartHours } from 'e-teleport/services/upgradeWindow';

import type { UpgradeWindowStartHour } from 'e-teleport/services/upgradeWindow';

export const makeLabel = (startHour: UpgradeWindowStartHour): string => {
  return `${String(startHour).padStart(2, '0')}:00 (UTC)`;
};

const upgradeWindowOptions: Option<UpgradeWindowStartHour>[] =
  availableUpgradeWindowStartHours.map((window: UpgradeWindowStartHour) => ({
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
  const handleChange = (selected: Option<UpgradeWindowStartHour>) =>
    onSelectedWindowChange(selected.value);

  return (
    <Dialog
      onClose={onCancel}
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
  onSave: () => void;
  selectedWindow: UpgradeWindowStartHour;
  onSelectedWindowChange: (string) => void;
  attempt: Attempt;
};
