'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { artifactName, verifyEntries } = require('../scripts/package-vsix');

const validEntries = [
  'extension/package.json',
  'extension/extension.js',
  'extension/client.js',
  'extension/language-configuration.json',
  'extension/syntaxes/onsentamago.tmLanguage.json',
  'extension/readme.md',
  'extension/node_modules/vscode-languageclient/package.json'
];

test('package contract has a stable artifact name and production contents', () => {
  assert.equal(artifactName, 'onsentamago-0.0.1.vsix');
  assert.doesNotThrow(() => verifyEntries(validEntries));
});

test('package contract rejects development-only contents', () => {
  for (const entry of [
    'extension/test/extension.test.js',
    'extension/scripts/package-vsix.js',
    'extension/node_modules/@vscode/vsce/package.json',
    'extension/package-lock.json'
  ]) {
    assert.throws(() => verifyEntries([...validEntries, entry]));
  }
});
