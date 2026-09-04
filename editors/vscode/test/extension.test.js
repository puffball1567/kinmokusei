'use strict';

const assert = require('node:assert/strict');
const Module = require('node:module');
const test = require('node:test');

const extensionPath = require.resolve('../extension');

test('extension entry point delegates activation and deactivation', async (t) => {
  const originalLoad = Module._load;
  const events = [];
  const watcher = { kind: 'watcher' };
  const commandRegistration = { kind: 'command-registration' };
  const configurationRegistration = { kind: 'configuration-registration' };

  class LanguageClient {
    constructor(id, name, options, clientOptions) {
      events.push('construct');
      assert.equal(id, 'kinmokusei');
      assert.equal(name, 'Kinmokusei Language Server');
      assert.equal(options.run.command, 'keika');
      assert.deepEqual(options.run.args, ['lsp', '--stdio']);
      assert.deepEqual(clientOptions.documentSelector, [
        { scheme: 'file', language: 'kinmokusei' }
      ]);
    }

    async start() { events.push('start'); }
    async stop() { events.push('stop'); }
  }

  const vscode = {
    workspace: {
      workspaceFolders: [],
      getConfiguration(section) {
        assert.equal(section, 'kinmokusei');
        return { get: (_name, fallback) => fallback };
      },
      createFileSystemWatcher(pattern) {
        assert.equal(pattern, '**/*.km');
        return watcher;
      },
      onDidChangeConfiguration() { return configurationRegistration; }
    },
    commands: {
      registerCommand(command) {
        assert.equal(command, 'kinmokusei.restartLanguageServer');
        return commandRegistration;
      }
    },
    window: {
      async showErrorMessage(message) {
        assert.fail('unexpected startup error: ' + message);
      }
    }
  };

  Module._load = function load(request, parent, isMain) {
    if (request === 'vscode') {
      events.push('load:vscode');
      return vscode;
    }
    if (request === 'vscode-languageclient/node') {
      events.push('load:languageclient');
      return { LanguageClient };
    }
    return originalLoad.call(this, request, parent, isMain);
  };
  t.after(() => {
    Module._load = originalLoad;
    delete require.cache[extensionPath];
  });

  delete require.cache[extensionPath];
  const extension = require(extensionPath);
  const context = { subscriptions: [] };

  await extension.activate(context);
  assert.deepEqual(context.subscriptions, [
    watcher, commandRegistration, configurationRegistration
  ]);
  assert.deepEqual(events, [
    'load:vscode', 'load:languageclient', 'construct', 'start'
  ]);

  await extension.deactivate();
  await extension.deactivate();
  assert.deepEqual(events, [
    'load:vscode', 'load:languageclient', 'construct', 'start', 'stop'
  ]);
});
