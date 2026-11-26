import test from 'node:test';
import assert from 'node:assert/strict';

import { migrateStorage } from './storage-migrate.js';

function memStore(init) {
  const m = new Map(Object.entries(init));
  return {
    get length() { return m.size; },
    key: (i) => [...m.keys()][i] ?? null,
    getItem: (k) => (m.has(k) ? m.get(k) : null),
    setItem: (k, v) => m.set(k, String(v)),
    removeItem: (k) => m.delete(k),
    dump: () => Object.fromEntries(m),
  };
}

test('px0.* keys move to hos-code.* once, a newer hos-code value wins, other keys stay', () => {
  const s = memStore({ 'px0.theme': 'dark', 'px0.wrap': 'true', 'hos-code.wrap': 'false', other: 'x' });
  migrateStorage(s);
  assert.deepEqual(s.dump(), { 'hos-code.wrap': 'false', other: 'x', 'hos-code.theme': 'dark' });
  migrateStorage(s);
  assert.deepEqual(s.dump(), { 'hos-code.wrap': 'false', other: 'x', 'hos-code.theme': 'dark' });
});
