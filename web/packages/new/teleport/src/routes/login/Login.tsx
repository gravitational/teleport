import { NewLoginForm } from './LoginForm';

export function NewLoginRoute() {
  // const [licenseAcknowledged] = useLocalStorage(
  //   KeysEnum.LICENSE_ACKNOWLEDGED,
  //   false
  // );
  //
  // const checkingValidSession = useCheckSessionAndRedirect();
  //
  // if (checkingValidSession) {
  //   return null;
  // }
  //
  // if (!licenseAcknowledged) {
  //   return null;
  // }
  //
  // return <div>hello there!</div>;

  return <NewLoginForm />;
}
