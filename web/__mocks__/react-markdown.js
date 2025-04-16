// Manually mocks react-markdown for testing due to ES modules
// https://jestjs.io/docs/manual-mocks
function ReactMarkdown({ children }) {
  return <>{children}</>;
}

export default ReactMarkdown;
