/** FormDataField defines the hardcoded form field
 * names expected by the backend.
 */
export enum FormDataField {
  // This is snake_case due to existing web handler's formdata values.
  FallbackChannel = 'fallback_channel',

  // Bot token is used only for static credentials enrollment.
  BotToken = 'botToken',
}
