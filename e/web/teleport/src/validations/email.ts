/**
 * Returns true if the string is a valid email
 *
 * @remarks
 * is formatted as chars<@>chars<at least one '.'>chars where chars is not empty
 * it does accept a slim number of invalid email addresses, such as:
 * 'char.@char.com', '.char@char.com', 'char..char@char.char'
 *
 * @param email - string value to be validated
 * @returns boolean value of validation
 */
export const isValidEmail = (email: string): boolean => {
  // emailParts returns [local, domain]
  // e.x.: test@example.com = ['test', 'example.com']
  const emailParts = email.split('@');
  if (emailParts.length !== 2 || !emailParts.every(notEmpty)) {
    return false;
  }

  // domainParts returns at minimum [host, dns]
  // e.x.: example.com = ['example', 'com']; example.ex.com = ['example', 'ex', 'com'
  const domainParts = emailParts[1].split('.');
  return !(domainParts.length < 2 || !domainParts.every(notEmpty));
};

const notEmpty = (value: string) => value != '';
