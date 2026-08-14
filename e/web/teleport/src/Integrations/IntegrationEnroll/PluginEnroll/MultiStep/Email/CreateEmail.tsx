import { usePlugin } from '../../MultiStep/usePlugin';
import { SubmittablePluginForm } from '../../SubmittablePluginForm';

export function CreateEmail() {
  const { selectedPlugin, eventId, setFormData, nextStep } = usePlugin();

  function handleSetFormData(formData: FormData) {
    setFormData(formData);
    nextStep();
  }

  return (
    <SubmittablePluginForm
      plugin={selectedPlugin}
      eventId={eventId}
      setFormData={handleSetFormData}
    />
  );
}
