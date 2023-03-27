export default function parseError(json) {
  let msg = '';

  if (json && json.error) {
    msg = json.error.message;
  } else if (json && json.message) {
    msg = json.message;
  } else if (json.responseText) {
    msg = json.responseText;
  }
  return msg;
}

export class ApiError extends Error {
  constructor(message, response) {
    message = message || 'Unknown error';
    super(message);
    this.response = response;
    this.name = 'ApiError';
  }
}
