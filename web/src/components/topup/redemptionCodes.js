export const parseRedemptionCodes = (value) => {
  return value
    .split(/\r\n|\n|\r/)
    .map((code) => code.trim())
    .filter(Boolean);
};
