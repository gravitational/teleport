import hljs from 'highlight.js/lib/core';
import sql from 'highlight.js/lib/languages/sql';
import { useEffect, useRef, type PropsWithChildren } from 'react';

hljs.registerLanguage('sql', sql);

interface HighlightProps {
  className?: string;
}

export function Highlight({
  className,
  children,
}: PropsWithChildren<HighlightProps>) {
  const ref = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (ref.current) {
      const code = ref.current.querySelector('code');

      if (!code) {
        return;
      }

      hljs.highlightBlock(code);
    }
  }, []);

  return (
    <pre ref={ref}>
      <code className={className}>{children}</code>
    </pre>
  );
}
