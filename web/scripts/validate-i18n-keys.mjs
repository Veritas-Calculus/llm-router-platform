#!/usr/bin/env node
/**
 * validate-i18n-keys.mjs — fail CI when locale JSON files diverge in key shape.
 *
 * The custom t() in src/lib/i18n.ts silently falls back to English on a missing
 * key, so Chinese users were seeing English fragments in the Settings/MFA
 * pages (35 keys at the time of the audit). This script enumerates the leaf
 * keys in every locale file and reports any key present in one and missing in
 * another. Non-zero exit fails the build.
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const LOCALES_DIR = path.resolve(__dirname, '..', 'src', 'locales');

// Walk a JSON object and emit dot-paths to every leaf (non-object) value.
function collectLeafKeys(obj, prefix = '', out = new Set()) {
  for (const [k, v] of Object.entries(obj)) {
    const dotted = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
      collectLeafKeys(v, dotted, out);
    } else {
      out.add(dotted);
    }
  }
  return out;
}

function loadLocale(file) {
  const text = fs.readFileSync(path.join(LOCALES_DIR, file), 'utf8');
  return JSON.parse(text);
}

const localeFiles = fs.readdirSync(LOCALES_DIR).filter((f) => f.endsWith('.json'));
if (localeFiles.length < 2) {
  console.log(`Only ${localeFiles.length} locale file(s); nothing to compare.`);
  process.exit(0);
}

const keysByLocale = new Map();
for (const file of localeFiles) {
  const locale = file.replace(/\.json$/, '');
  keysByLocale.set(locale, collectLeafKeys(loadLocale(file)));
}

// Treat the file with the most keys as the reference and report holes in others.
// (We don't pick one locale as canonical — every locale must match every other.)
const allKeys = new Set();
for (const keys of keysByLocale.values()) {
  for (const k of keys) allKeys.add(k);
}

let problems = 0;
for (const [locale, keys] of keysByLocale) {
  const missing = [...allKeys].filter((k) => !keys.has(k)).sort();
  if (missing.length > 0) {
    problems += missing.length;
    console.error(`\n[${locale}.json] missing ${missing.length} key(s):`);
    for (const k of missing) console.error(`  - ${k}`);
  }
}

if (problems > 0) {
  console.error(`\nFAIL: ${problems} key parity issue(s) across ${localeFiles.length} locale files.`);
  console.error('Add the missing translations or remove the key from the locale(s) that have it.');
  process.exit(1);
}

console.log(`OK: ${allKeys.size} keys consistent across ${localeFiles.length} locale files.`);
