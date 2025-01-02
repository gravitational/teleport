import { ButtonSecondary, Text } from 'design';
import Dialog, { DialogContent, DialogFooter } from 'design/Dialog';
import { ShieldCheck } from 'design/Icon';

import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';

export default function FinishedReview({
  nextAuditDate,
  onClick,
}: {
  nextAuditDate: Date;
  onClick(): void;
}) {
  const date = getFormattedDate(nextAuditDate);
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
        textAlign: 'center',
      })}
      open={true}
    >
      <DialogContent alignItems="center">
        <ShieldCheck size={48} mb={3} />
        <Text mb={1}>
          Your review has been successfully submitted.
          {date && (
            <>
              <br />
              Next review date is: {date}
            </>
          )}
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClick}>OK</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
