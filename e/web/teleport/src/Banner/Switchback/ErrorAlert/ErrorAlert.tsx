import { ButtonSecondary, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

export default function BannerError({ err, onClose }: Props) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>An Error has Occurred</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Alert kind="danger" children={err} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose} mr={3}>
          OK
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  err: string;
  onClose(): void;
};
