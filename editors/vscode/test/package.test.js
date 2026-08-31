'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');
const path = require('node:path');
const { artifactName, productionDependencyDirectories, verifyEntries } = require('../scripts/package-vsix');

const validEntries = [
  'extension/package.json',
  'extension/extension.js',
  'extension/client.js',
  'extension/language-configuration.json',
  'extension/syntaxes/onsentamago.tmLanguage.json',
  'extension/readme.md',
  'extension/LICENSE.txt',
  'extension/node_modules/vscode-languageclient/package.json'
];

test('package contract has a stable artifact name and production contents', () => {
  assert.equal(artifactName, 'onsentamago-0.1.0.vsix');
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

test('package dependency set comes from production lock entries', () => {
  const root = path.resolve(__dirname, '..');
  const relative = productionDependencyDirectories(root)
    .slice(1)
    .map((directory) => path.relative(root, directory))
    .sort();
  assert.deepEqual(relative, [
    'node_modules/balanced-match',
    'node_modules/brace-expansion',
    'node_modules/minimatch',
    'node_modules/semver',
    'node_modules/vscode-jsonrpc',
    'node_modules/vscode-languageclient',
    'node_modules/vscode-languageserver-protocol',
    'node_modules/vscode-languageserver-textdocument',
    'node_modules/vscode-languageserver-types'
  ]);
});

test('extension and repository license texts stay identical', () => {
  const root = path.resolve(__dirname, '..');
  assert.equal(
    fs.readFileSync(path.join(root, 'LICENSE'), 'utf8'),
    fs.readFileSync(path.join(root, '..', '..', 'LICENSE'), 'utf8')
  );
});
