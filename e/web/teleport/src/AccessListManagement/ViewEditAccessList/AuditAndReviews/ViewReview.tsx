import { ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import TextEditor from 'shared/components/TextEditor';

import { AccessListReview } from 'e-teleport/services/accessmanagement';

export function ViewReview({
  review,
  onClose,
}: {
  review: AccessListReview;
  onClose(): void;
}) {
  const json = JSON.stringify(review.raw, null, 2);

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Review Changes</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <TextEditor readOnly={true} data={[{ content: json, type: 'json' }]} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

const dialogCss = () => `
  min-height: 100px;
  max-height: 80%;
  height: 100%;
  min-width: 100px;
  max-width: 600px;
  width: 100%;
`;
