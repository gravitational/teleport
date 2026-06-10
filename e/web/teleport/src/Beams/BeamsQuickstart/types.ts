import { ComponentType, ReactNode } from 'react';

import { IconProps } from 'design/Icon/Icon';

export type SectionId =
  | 'welcome'
  | 'tutorial-run'
  | 'tutorial-vibe'
  | 'tutorial-examples'
  | 'resources';

export type SectionDef = {
  id: SectionId;
  group: string;
  tocLabel?: string;
  title: string;
  cards: CardDef[];
};

export type CardDef = {
  id?: string;
  icon: ComponentType<IconProps>;
  steps?: StepDef[];
  trailing?: Block[];
};

export type StepDef = {
  eyebrow?: string;
  title?: ReactNode;
  blocks?: Block[];
};

export type Block =
  | { kind: 'code'; cmd: string; prompt?: string }
  | { kind: 'text'; body: ReactNode }
  | { kind: 'tips'; items: Array<{ name: string; body: ReactNode }> }
  | { kind: 'reveal'; hint: ReactNode; cmd: string }
  | {
      kind: 'radios';
      name: string;
      options: RadioOption[];
      defaultValue?: string;
    }
  | {
      kind: 'link';
      label: string;
      href: string;
      icon: ComponentType<IconProps>;
    }
  | { kind: 'faqs'; items: Array<{ q: string; a: ReactNode }> };

export type RadioOption = {
  value: string;
  label: string;
  // Command to display when this option is selected.
  cmd: string;
};
