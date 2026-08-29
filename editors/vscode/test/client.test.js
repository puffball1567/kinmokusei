'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const {
  DEFAULT_SERVER_PATH, LANGUAGE_ID, RESTART_COMMAND, SERVER_ARGUMENTS,
  createController, normalizeServerPath, serverOptions, workspaceDirectory
} = require('../client');

function mockEnvironment() {
  const events = [];
  const instances = [];
  const errors = [];
  const subscriptions = [];
  let configuredPath = DEFAULT_SERVER_PATH;
  let failNextStart = false;
  let configurationListener;
  let restartCommand;

  class LanguageClient {
    constructor(id, name, options, clientOptions) {
      Object.assign(this, { id, name, options, clientOptions });
      this.number = instances.length;
      this.disposed = false;
      instances.push(this);
    }
    async start() {
      events.push('start:' + this.number);
      if (failNextStart) {
        failNextStart = false;
        throw new Error('spawn failed');
      }
    }
    async stop() { events.push('stop:' + this.number); }
    async dispose() {
      this.disposed = true;
      events.push('dispose:' + this.number);
    }
  }

  const watcher = { kind: 'watcher' };
  const vscode = {
    workspace: {
      workspaceFolders: [
        { uri: { scheme: 'untitled', fsPath: '/ignored' } },
        { uri: { scheme: 'file', fsPath: '/workspace' } }
      ],
      getConfiguration(section) {
        assert.equal(section, 'onsentamago');
        return { get(name, fallback) {
          assert.equal(name, 'server.path');
          return configuredPath === undefined ? fallback : configuredPath;
        } };
      },
      createFileSystemWatcher(pattern) {
        events.push('watch:' + pattern);
        return watcher;
      },
      onDidChangeConfiguration(listener) {
        configurationListener = listener;
        return { kind: 'configuration-registration' };
      }
    },
    commands: {
      registerCommand(command, callback) {
        assert.equal(command, RESTART_COMMAND);
        restartCommand = callback;
        return { kind: 'command-registration' };
      }
    },
    window: {
      async showErrorMessage(message) { errors.push(message); }
    }
  };

  return {
    LanguageClient, errors, events, instances,
    context: { subscriptions }, subscriptions, vscode, watcher,
    changePath(value) {
      configuredPath = value;
      configurationListener({
        affectsConfiguration(name) {
          return name === 'onsentamago.server.path';
        }
      });
    },
    failStart() { failNextStart = true; },
    restart() { return restartCommand(); }
  };
}

async function eventually(predicate) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (predicate()) return;
    await new Promise((resolve) => setImmediate(resolve));
  }
  assert.fail('condition was not reached');
}

test('server path normalization matrix', () => {
  for (const [input, want] of [
    [undefined, DEFAULT_SERVER_PATH], [null, DEFAULT_SERVER_PATH],
    ['', DEFAULT_SERVER_PATH], ['  ', DEFAULT_SERVER_PATH],
    [' ontama-dev ', 'ontama-dev'], ['/opt/ontama', '/opt/ontama']
  ]) {
    assert.equal(normalizeServerPath(input), want);
  }
  for (const input of [0, false, {}, []]) {
    assert.throws(() => normalizeServerPath(input), /must be a string/);
  }
  assert.throws(() => normalizeServerPath('bad\0path'), /must not contain NUL/);
});

test('server options preserve the fixed stdio protocol contract', () => {
  const options = serverOptions('/tools/ontama', '/workspace');
  assert.deepEqual(options.run, {
    command: '/tools/ontama',
    args: ['lsp', '--stdio'],
    options: { cwd: '/workspace' }
  });
  assert.deepEqual(options.debug, options.run);
  assert.notStrictEqual(options.run, options.debug);
  assert.notStrictEqual(options.run.args, options.debug.args);
  assert.deepEqual(SERVER_ARGUMENTS, ['lsp', '--stdio']);
  const withoutWorkspace = serverOptions('ontama');
  assert.equal(Object.hasOwn(withoutWorkspace.run, 'options'), false);
  assert.equal(withoutWorkspace.debug.options, undefined);
});

test('workspace selection uses the first usable file folder', () => {
  const environment = mockEnvironment();
  assert.equal(workspaceDirectory(environment.vscode), '/workspace');
  environment.vscode.workspace.workspaceFolders = [];
  assert.equal(workspaceDirectory(environment.vscode), undefined);
  environment.vscode.workspace.workspaceFolders = [
    null,
    { uri: { scheme: 'file', fsPath: '' } },
    { uri: { scheme: 'vscode-remote', fsPath: '/remote' } }
  ];
  assert.equal(workspaceDirectory(environment.vscode), undefined);
});

test('activation starts exactly one file-only language client', async () => {
  const environment = mockEnvironment();
  const controller = createController(environment);
  assert.equal(await controller.activate(environment.context), true);
  assert.deepEqual(controller.state(), { activated: true, running: true });
  assert.equal(environment.instances.length, 1);
  assert.deepEqual(environment.instances[0].options.run, {
    command: 'ontama',
    args: ['lsp', '--stdio'],
    options: { cwd: '/workspace' }
  });
  assert.deepEqual(environment.instances[0].clientOptions.documentSelector, [
    { scheme: 'file', language: LANGUAGE_ID }
  ]);
  assert.equal(environment.instances[0].clientOptions.synchronize.fileEvents, environment.watcher);
  assert.equal(environment.instances[0].clientOptions.outputChannelName, 'OnsenTamago');
  assert.deepEqual(environment.events, ['watch:**/*.otm', 'start:0']);
  assert.equal(environment.subscriptions.length, 3);
  assert.equal(await controller.activate(environment.context), true);
  assert.equal(environment.instances.length, 1);
  assert.equal(environment.subscriptions.length, 3);
});

test('manual and configuration restarts are serialized and use current configuration', async () => {
  const environment = mockEnvironment();
  const controller = createController(environment);
  await controller.activate(environment.context);
  await environment.restart();
  assert.deepEqual(environment.events.slice(-2), ['stop:0', 'start:1']);
  environment.changePath(' /tools/ontama-next ');
  await eventually(() => environment.instances.length === 3);
  assert.deepEqual(environment.events.slice(-2), ['stop:1', 'start:2']);
  assert.equal(environment.instances[2].options.run.command, '/tools/ontama-next');
});

test('startup failure is reported, disposed, and retryable', async () => {
  const environment = mockEnvironment();
  const controller = createController(environment);
  environment.failStart();
  assert.equal(await controller.activate(environment.context), false);
  assert.deepEqual(controller.state(), { activated: true, running: false });
  assert.equal(environment.instances[0].disposed, true);
  assert.match(environment.errors[0], /Unable to start.*ontama.*spawn failed/);
  assert.equal(await controller.restart(), true);
  assert.deepEqual(controller.state(), { activated: true, running: true });
  assert.deepEqual(environment.events, [
    'watch:**/*.otm', 'start:0', 'dispose:0', 'start:1'
  ]);
});

test('invalid configuration is visible and remains retryable', async () => {
  const environment = mockEnvironment();
  const controller = createController(environment);
  environment.vscode.workspace.getConfiguration = () => ({ get: () => 'bad\0path' });
  assert.equal(await controller.activate(environment.context), false);
  assert.equal(environment.instances.length, 0);
  assert.match(environment.errors[0], /must not contain NUL/);
  assert.deepEqual(controller.state(), { activated: true, running: false });
});

test('deactivation stops once and is idempotent', async () => {
  const environment = mockEnvironment();
  const controller = createController(environment);
  await controller.activate(environment.context);
  await controller.deactivate();
  await controller.deactivate();
  assert.deepEqual(environment.events, ['watch:**/*.otm', 'start:0', 'stop:0']);
  assert.deepEqual(controller.state(), { activated: false, running: false });
});
