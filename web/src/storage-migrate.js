// web/src/storage-migrate.js
export const STORAGE_PREFIX = 'hos-code.';
const LEGACY_PREFIX = 'px0.';

export function migrateStorage(store) {
  const keys = [];
  for (let i = 0; i < store.length; i++) {
    const key = store.key(i);
    if (key && key.startsWith(LEGACY_PREFIX)) keys.push(key);
  }
  for (const key of keys) {
    const next = STORAGE_PREFIX + key.slice(LEGACY_PREFIX.length);
    if (store.getItem(next) === null) store.setItem(next, store.getItem(key));
    store.removeItem(key);
  }
}

try { migrateStorage(localStorage); } catch {}
