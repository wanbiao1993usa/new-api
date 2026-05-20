import test from 'node:test';
import assert from 'node:assert/strict';

import { parseRedemptionCodes } from './redemptionCodes.js';

test('parseRedemptionCodes splits by line and ignores blank lines', () => {
  assert.deepEqual(parseRedemptionCodes(' code-a \n\ncode-b\r\n code-c '), [
    'code-a',
    'code-b',
    'code-c',
  ]);
});
