import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import { Markdown } from './Markdown';

function renderMarkdown(text: string) {
  return render(<Markdown text={text} />);
}

describe('Markdown', () => {
  describe('inline formatting', () => {
    it('renders bold text', () => {
      renderMarkdown(`This is **bold** text`);

      expect(screen.getByText('bold')).toBeInTheDocument();
      expect(screen.getByText('bold').tagName).toBe('STRONG');
    });

    it('renders inline code', () => {
      renderMarkdown(`This is \`code\` text`);

      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getByText('code').tagName).toBe('CODE');
    });

    it('renders links', () => {
      renderMarkdown(`This is [a link](https://example.com) text`);

      const link = screen.getByRole('link', { name: 'a link' });

      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://example.com');
    });

    it('renders multiple inline elements', () => {
      renderMarkdown(`**bold** and \`code\` and [link](https://example.com) together`);

      expect(screen.getByText('bold')).toBeInTheDocument();
      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'link' })).toBeInTheDocument();
    });

    it('handles nested markers correctly', () => {
      renderMarkdown(`**bold \`code\` text**`);

      const strong = screen.getByText((content, element) => {
        return element.tagName === 'STRONG' && element.textContent === 'bold code text';
      });

      expect(strong).toBeInTheDocument();

      const code = screen.getByText('code');

      expect(code).toBeInTheDocument();

      expect(code.tagName).toBe('CODE');
    });

    it('handles links with special characters in URL', () => {
      renderMarkdown(`[link](https://example.com/path?query=1&foo=bar#hash)`);

      const link = screen.getByRole('link', { name: 'link' });

      expect(link).toHaveAttribute(
        'href',
        'https://example.com/path?query=1&foo=bar#hash'
      );
    });

    it('renders links with inline formatting in text', () => {
      renderMarkdown(`[**bold** link](https://example.com)`);

      const link = screen.getByRole('link', { name: '**bold** link' });

      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://example.com');
    });
  });

  describe('headers', () => {
    it('renders h1', () => {
      renderMarkdown(`# Header 1`);

      expect(
        screen.getByRole('heading', {
          level: 1,
          name: 'Header 1',
        })
      ).toBeInTheDocument();
    });

    it('renders h2 through h6', () => {
      const text = `## Header 2
### Header 3
#### Header 4
##### Header 5
###### Header 6`;

      renderMarkdown(text);

      expect(
        screen.getByRole('heading', {
          level: 2,
          name: 'Header 2',
        })
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', {
          level: 3,
          name: 'Header 3',
        })
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', {
          level: 4,
          name: 'Header 4',
        })
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', {
          level: 5,
          name: 'Header 5',
        })
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', {
          level: 6,
          name: 'Header 6',
        })
      ).toBeInTheDocument();
    });

    it('trims header content', () => {
      renderMarkdown(`#   Spaced Header`);

      expect(
        screen.getByRole('heading', {
          level: 1,
          name: 'Spaced Header',
        })
      ).toBeInTheDocument();
    });
  });

  describe('lists', () => {
    it('renders unordered list', () => {
      const text = `- Item 1
- Item 2
- Item 3`;

      renderMarkdown(text);

      expect(screen.getByRole('list')).toBeInTheDocument();

      const items = screen.getAllByRole('listitem');

      expect(items).toHaveLength(3);

      expect(items[0]).toHaveTextContent('Item 1');
      expect(items[1]).toHaveTextContent('Item 2');
      expect(items[2]).toHaveTextContent('Item 3');
    });

    it('renders list with inline formatting', () => {
      const text = `- **Bold** item
- Item with \`code\`
- Item with [link](https://example.com)`;

      renderMarkdown(text);

      expect(screen.getByText('Bold')).toBeInTheDocument();
      expect(screen.getByText('Bold').tagName).toBe('STRONG');
      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getByText('code').tagName).toBe('CODE');
      expect(screen.getByRole('link', { name: 'link' })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'link' })).toHaveAttribute(
        'href',
        'https://example.com'
      );
    });

    it('stops list when non-list line encountered', () => {
      const text = `- Item 1
- Item 2
Regular paragraph`;

      renderMarkdown(text);

      const items = screen.getAllByRole('listitem');

      expect(items).toHaveLength(2);

      expect(screen.getByText('Regular paragraph')).toBeInTheDocument();
    });
  });

  describe('paragraphs', () => {
    it('renders single line paragraph', () => {
      renderMarkdown(`This is a paragraph`);

      expect(screen.getByText('This is a paragraph')).toBeInTheDocument();
    });

    it('joins multiple lines into single paragraph', () => {
      const text = `This is line one
This is line two
This is line three`;

      renderMarkdown(text);

      expect(
        screen.getByText('This is line one This is line two This is line three')
      ).toBeInTheDocument();
    });

    it('separates paragraphs by empty lines', () => {
      const text = `First paragraph

Second paragraph`;

      renderMarkdown(text);

      expect(screen.getByText('First paragraph')).toBeInTheDocument();
      expect(screen.getByText('Second paragraph')).toBeInTheDocument();
    });

    it('renders paragraph with inline formatting', () => {
      renderMarkdown(`Paragraph with **bold** and \`code\` and [link](https://example.com)`);

      expect(screen.getByText('bold')).toBeInTheDocument();
      expect(screen.getByText('bold').tagName).toBe('STRONG');
      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getByText('code').tagName).toBe('CODE');
      expect(screen.getByRole('link', { name: 'link' })).toBeInTheDocument();
    });
  });

  describe('mixed content', () => {
    it('renders complete document', () => {
      const text = `# Main Title

This is a paragraph with **bold** text and [a link](https://example.com).

## Subsection

Here is a list:
- First item
- Second item with \`code\`
- Third item with [link](https://example.com)

Another paragraph after the list.`;

      renderMarkdown(text);

      expect(
        screen.getByRole('heading', {
          level: 1,
          name: 'Main Title',
        })
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', {
          level: 2,
          name: 'Subsection',
        })
      ).toBeInTheDocument();
      expect(
        screen.getByText('This is a paragraph with', { exact: false })
      ).toBeInTheDocument();
      expect(screen.getByText('Here is a list:')).toBeInTheDocument();
      expect(
        screen.getByText('Another paragraph after the list.')
      ).toBeInTheDocument();
      expect(screen.getAllByRole('listitem')).toHaveLength(3);
      expect(screen.getByText('bold')).toBeInTheDocument();
      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getAllByRole('link')).toHaveLength(2);
    });

    it('handles paragraph ending at header', () => {
      const text = `This is a paragraph
that continues
# New Header`;

      renderMarkdown(text);

      expect(
        screen.getByText('This is a paragraph that continues')
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', {
          level: 1,
          name: 'New Header',
        })
      ).toBeInTheDocument();
    });

    it('handles paragraph ending at list', () => {
      const text = `This is a paragraph
that continues
- List item`;

      renderMarkdown(text);

      expect(
        screen.getByText('This is a paragraph that continues')
      ).toBeInTheDocument();
      expect(screen.getByRole('listitem')).toHaveTextContent('List item');
    });
  });

  describe('edge cases', () => {
    it('handles empty string', () => {
      renderMarkdown(``);

      expect(screen.queryByRole('heading')).not.toBeInTheDocument();
      expect(screen.queryByRole('list')).not.toBeInTheDocument();
      expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
      expect(screen.queryByText(/.+/)).not.toBeInTheDocument();
    });

    it('handles only whitespace', () => {
      renderMarkdown(`\n\n\n`);

      expect(screen.queryByRole('heading')).not.toBeInTheDocument();
      expect(screen.queryByRole('list')).not.toBeInTheDocument();
      expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });

    it('handles multiple consecutive empty lines', () => {
      const text = `Paragraph 1


Paragraph 2`;

      renderMarkdown(text);

      expect(screen.getByText('Paragraph 1')).toBeInTheDocument();
      expect(screen.getByText('Paragraph 2')).toBeInTheDocument();
    });

    it('ignores list items without space after dash', () => {
      const text = `-No space
- With space`;

      renderMarkdown(text);

      const items = screen.getAllByRole('listitem');
      expect(items).toHaveLength(1);
      expect(items[0]).toHaveTextContent('With space');
    });

    it('handles unclosed inline markers', () => {
      renderMarkdown(`**unclosed bold`);

      expect(screen.queryByText('bold')).not.toBeInTheDocument();
      expect(screen.getByText('**unclosed bold')).toBeInTheDocument();
    });

    it('handles unclosed links', () => {
      renderMarkdown(`[unclosed link`);

      expect(screen.queryByRole('link')).not.toBeInTheDocument();
      expect(screen.getByText('[unclosed link')).toBeInTheDocument();
    });

    it('handles links without URL', () => {
      renderMarkdown(`[link text]()`);

      const link = screen.getByText('link text');

      expect(link).toHaveAttribute('href', '');
    });

    it('handles empty link text', () => {
      renderMarkdown(`[](https://example.com)`);

      const link = screen.getByRole('link', { name: '' });

      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://example.com');
    });
  });

  describe('memoization', () => {
    it('returns same result for same input', () => {
      const text = '# Header\n\nParagraph with [link](https://example.com)';

      const { rerender } = renderMarkdown(text);

      expect(
        screen.getByRole('heading', {
          level: 1,
          name: 'Header',
        })
      ).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'link' })).toBeInTheDocument();

      const heading = screen.getByRole('heading', { level: 1 });
      const link = screen.getByRole('link');

      rerender(<Markdown text={text} />);

      expect(screen.getByRole('heading', { level: 1 })).toBe(heading);
      expect(screen.getByRole('link')).toBe(link);
    });

    it('updates when text changes', () => {
      const { rerender } = renderMarkdown(`Original`);

      expect(screen.getByText('Original')).toBeInTheDocument();

      rerender(<Markdown text="Updated" />);

      expect(screen.queryByText('Original')).not.toBeInTheDocument();
      expect(screen.getByText('Updated')).toBeInTheDocument();
    });
  });
});
