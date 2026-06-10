import { useState } from 'react';

import { ButtonSecondary, Flex } from 'design';
import { CollapsibleInfoSection } from 'design/CollapsibleInfoSection';
import { RadioGroup } from 'design/RadioGroup';
import { P2 } from 'design/Text';

import { CodeBlock } from './components/CodeBlock';
import { RevealCommand } from './components/RevealCommand';
import { Tip, TipList } from './components/Tip';
import { Block, RadioOption } from './types';

// Single dispatcher that turns a Block data object into the matching
// component tree.
export function renderBlock(block: Block, key: number) {
  switch (block.kind) {
    case 'code':
      return <CodeBlock key={key} command={block.cmd} prompt={block.prompt} />;
    case 'text':
      return (
        <P2 key={key} color="text.slightlyMuted">
          {block.body}
        </P2>
      );
    case 'tips':
      return (
        <TipList key={key}>
          {block.items.map(t => (
            <Tip key={t.name} label={t.name}>
              {t.body}
            </Tip>
          ))}
        </TipList>
      );
    case 'reveal':
      return <RevealCommand key={key} hint={block.hint} command={block.cmd} />;
    case 'radios':
      return (
        <RadiosBlock
          key={key}
          name={block.name}
          options={block.options}
          defaultValue={block.defaultValue}
        />
      );
    case 'link': {
      const Icon = block.icon;
      return (
        <ButtonSecondary
          key={key}
          as="a"
          href={block.href}
          target="_blank"
          rel="noopener noreferrer"
          gap={2}
          css={`
            align-self: flex-start;
          `}
        >
          {block.label}
          <Icon size={16} />
        </ButtonSecondary>
      );
    }
    case 'faqs':
      return (
        <Flex
          key={key}
          flexDirection="column"
          gap={2}
          css={`
            align-self: stretch;
          `}
        >
          {block.items.map(item => (
            <CollapsibleInfoSection
              key={item.q}
              size="small"
              openLabel={item.q}
              closeLabel={item.q}
            >
              {item.a}
            </CollapsibleInfoSection>
          ))}
        </Flex>
      );
  }
}

// The only stateful Block. Empty string default keeps the radios
// controlled from the first render, if a defaultValue is supplied (e.g.
// inferred from the current user's auth setup) the matching command is
// shown immediately. Either way the user can still pick the other option.
function RadiosBlock({
  name,
  options,
  defaultValue,
}: {
  name: string;
  options: RadioOption[];
  defaultValue?: string;
}) {
  const [value, setValue] = useState<string>(defaultValue ?? '');
  const selected = options.find(o => o.value === value);
  return (
    <Flex flexDirection="column" gap={3}>
      <RadioGroup
        name={name}
        options={options}
        value={value}
        onChange={setValue}
      />
      {selected && <CodeBlock command={selected.cmd} />}
    </Flex>
  );
}
