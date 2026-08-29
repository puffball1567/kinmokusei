'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const yauzl = require('yauzl');
const { createVSIX } = require('@vscode/vsce');

const extensionRoot = path.resolve(__dirname, '..');
const manifest = require(path.join(extensionRoot, 'package.json'));
const artifactName = `${manifest.name}-${manifest.version}.vsix`;

function sha256(file) {
  return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

function zipEntries(file) {
  return new Promise((resolve, reject) => {
    yauzl.open(file, { lazyEntries: true }, (openError, zip) => {
      if (openError) {
        reject(openError);
        return;
      }
      const entries = [];
      zip.on('error', reject);
      zip.on('entry', (entry) => {
        entries.push(entry.fileName);
        zip.readEntry();
      });
      zip.on('end', () => resolve(entries));
      zip.readEntry();
    });
  });
}

function verifyEntries(entries) {
  const required = [
    'extension/package.json',
    'extension/extension.js',
    'extension/client.js',
    'extension/language-configuration.json',
    'extension/syntaxes/onsentamago.tmLanguage.json',
    'extension/readme.md',
    'extension/node_modules/vscode-languageclient/package.json'
  ];
  for (const entry of required) {
    assert.ok(entries.includes(entry), `VSIX is missing ${entry}`);
  }
  const forbiddenPrefixes = [
    'extension/test/',
    'extension/scripts/',
    'extension/dist/',
    'extension/node_modules/@vscode/'
  ];
  for (const prefix of forbiddenPrefixes) {
    assert.ok(
      !entries.some((entry) => entry.startsWith(prefix)),
      `VSIX unexpectedly contains ${prefix}`
    );
  }
  assert.ok(!entries.includes('extension/package-lock.json'));
}

async function createPackage(packagePath) {
  await createVSIX({
    cwd: extensionRoot,
    packagePath,
    useYarn: false,
    dependencies: true,
    allowMissingRepository: true,
    skipLicense: true
  });
}

async function main() {
  process.env.SOURCE_DATE_EPOCH = '315532800';
  const temporaryDirectory = fs.mkdtempSync(
    path.join(os.tmpdir(), 'onsentamago-vsix-')
  );
  const first = path.join(temporaryDirectory, 'first.vsix');
  const second = path.join(temporaryDirectory, 'second.vsix');
  try {
    await createPackage(first);
    await createPackage(second);
    verifyEntries(await zipEntries(first));
    assert.equal(
      sha256(first),
      sha256(second),
      'VSIX output is not byte-for-byte reproducible'
    );

    const outputDirectory = path.join(extensionRoot, 'dist');
    fs.mkdirSync(outputDirectory, { recursive: true });
    const output = path.join(outputDirectory, artifactName);
    fs.copyFileSync(first, output);
    console.log(`Created ${path.relative(extensionRoot, output)} (${sha256(output)})`);
  } finally {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
  }
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
}

module.exports = { artifactName, sha256, verifyEntries, zipEntries };
