import { Language } from './Language';

export interface FileProps {
  name: string;
  language: Language;
  code: string;
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function File(props: FileProps) {
  return null;
}
