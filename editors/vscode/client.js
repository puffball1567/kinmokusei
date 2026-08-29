'use strict';

const DEFAULT_SERVER_PATH = 'ontama';
const SERVER_ARGUMENTS = Object.freeze(['lsp', '--stdio']);
const LANGUAGE_ID = 'onsentamago';
const RESTART_COMMAND = 'onsentamago.restartLanguageServer';

function normalizeServerPath(value) {
  if (value === undefined || value === null) {
    return DEFAULT_SERVER_PATH;
  }
  if (typeof value !== 'string') {
    throw new TypeError('onsentamago.server.path must be a string');
  }
  const normalized = value.trim();
  if (normalized.includes('\0')) {
    throw new TypeError('onsentamago.server.path must not contain NUL');
  }
  return normalized || DEFAULT_SERVER_PATH;
}

function workspaceDirectory(vscode) {
  const folders = vscode.workspace.workspaceFolders || [];
  for (const folder of folders) {
    if (folder && folder.uri && folder.uri.scheme === 'file' && folder.uri.fsPath) {
      return folder.uri.fsPath;
    }
  }
  return undefined;
}

function serverOptions(command, cwd) {
  const executable = {
    command,
    args: [...SERVER_ARGUMENTS]
  };
  if (cwd) {
    executable.options = { cwd };
  }
  return {
    run: executable,
    debug: {
      ...executable,
      args: [...executable.args],
      options: executable.options ? { ...executable.options } : undefined
    }
  };
}

function createController({ vscode, LanguageClient }) {
  if (!vscode || typeof LanguageClient !== 'function') {
    throw new TypeError('vscode and LanguageClient are required');
  }

  let client;
  let watcher;
  let activated = false;
  let transition = Promise.resolve();

  function enqueue(operation) {
    const next = transition.then(operation, operation);
    transition = next.catch(() => undefined);
    return next;
  }

  function configuredServerPath() {
    const configuration = vscode.workspace.getConfiguration('onsentamago');
    return normalizeServerPath(configuration.get('server.path', DEFAULT_SERVER_PATH));
  }

  function makeClient() {
    const command = configuredServerPath();
    const options = serverOptions(command, workspaceDirectory(vscode));
    const clientOptions = {
      documentSelector: [{ scheme: 'file', language: LANGUAGE_ID }],
      synchronize: { fileEvents: watcher },
      outputChannelName: 'OnsenTamago'
    };
    return {
      command,
      instance: new LanguageClient(
        'onsentamago',
        'OnsenTamago Language Server',
        options,
        clientOptions
      )
    };
  }

  async function startInternal() {
    if (client) {
      return true;
    }
    let candidate;
    let command = DEFAULT_SERVER_PATH;
    try {
      const created = makeClient();
      command = created.command;
      candidate = created.instance;
      await candidate.start();
      client = candidate;
      return true;
    } catch (error) {
      if (candidate && typeof candidate.dispose === 'function') {
        try {
          await candidate.dispose();
        } catch {
          // Preserve the original startup error.
        }
      }
      const detail = error instanceof Error ? error.message : String(error);
      await vscode.window.showErrorMessage(
        'Unable to start OnsenTamago language server "' + command + '": ' +
          detail + '. Check onsentamago.server.path.'
      );
      return false;
    }
  }

  async function stopInternal() {
    const active = client;
    client = undefined;
    if (active) {
      await active.stop();
    }
  }

  async function restart() {
    return enqueue(async () => {
      await stopInternal();
      return startInternal();
    });
  }

  async function activate(context) {
    if (activated) {
      return true;
    }
    activated = true;
    watcher = vscode.workspace.createFileSystemWatcher('**/*.otm');
    const restartRegistration = vscode.commands.registerCommand(RESTART_COMMAND, restart);
    const configurationRegistration = vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration('onsentamago.server.path')) {
        restart().catch((error) => {
          const detail = error instanceof Error ? error.message : String(error);
          void vscode.window.showErrorMessage(
            'Unable to restart OnsenTamago language server: ' + detail
          );
        });
      }
    });
    context.subscriptions.push(watcher, restartRegistration, configurationRegistration);
    return enqueue(startInternal);
  }

  async function deactivate() {
    if (!activated) {
      return;
    }
    activated = false;
    await enqueue(stopInternal);
  }

  return {
    activate,
    deactivate,
    restart,
    state: () => ({ activated, running: Boolean(client) })
  };
}

module.exports = {
  DEFAULT_SERVER_PATH,
  LANGUAGE_ID,
  RESTART_COMMAND,
  SERVER_ARGUMENTS,
  createController,
  normalizeServerPath,
  serverOptions,
  workspaceDirectory
};
