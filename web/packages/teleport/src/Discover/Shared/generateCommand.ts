// generateCommand creates a command that will fetch and execute the given URL
export function generateCommand(url: string) {
  return `(Invoke-WebRequest -Uri ${url}).Content | Invoke-Expression`;
}
