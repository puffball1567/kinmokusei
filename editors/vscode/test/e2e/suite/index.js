'use strict';

const assert = require('node:assert/strict');
const path = require('node:path');
const vscode = require('vscode');

const EXTENSION_ID = 'kinmokusei.kinmokusei';
const HOVER_TEXT = 'function add(left: int, right: int): int';

async function eventually(operation, description, timeoutMilliseconds = 15000) {
  const deadline = Date.now() + timeoutMilliseconds;
  let lastValue;
  while (Date.now() < deadline) {
    lastValue = await operation();
    if (lastValue) {
      return lastValue;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.fail(description + '; last value: ' + JSON.stringify(lastValue));
}

function hoverText(hover) {
  return hover.contents.map((content) => {
    if (typeof content === 'string') {
      return content;
    }
    return content.value || '';
  }).join('\n');
}

async function requestExpectedHover(document) {
  const position = new vscode.Position(5, 22);
  return eventually(async () => {
    const results = await vscode.commands.executeCommand(
      'vscode.executeHoverProvider',
      document.uri,
      position
    );
    return results.find((hover) => hoverText(hover).includes(HOVER_TEXT));
  }, 'language server did not provide the expected hover');
}

async function replaceDocument(document, text) {
  const edit = new vscode.WorkspaceEdit();
  const end = document.positionAt(document.getText().length);
  edit.replace(document.uri, new vscode.Range(new vscode.Position(0, 0), end), text);
  assert.equal(await vscode.workspace.applyEdit(edit), true);
}

async function run() {
  const serverPath = process.env.KINMOKUSEI_E2E_SERVER_PATH;
  assert.ok(serverPath, 'KINMOKUSEI_E2E_SERVER_PATH is required');

  const configuration = vscode.workspace.getConfiguration('kinmokusei');
  const previousServerPath = configuration.inspect('server.path')?.workspaceValue;
  await configuration.update('server.path', serverPath, vscode.ConfigurationTarget.Workspace);

  const fixture = vscode.Uri.file(path.join(
    vscode.workspace.workspaceFolders[0].uri.fsPath,
    'main.km'
  ));
  const document = await vscode.workspace.openTextDocument(fixture);
  await vscode.window.showTextDocument(document);
  assert.equal(document.languageId, 'kinmokusei');

  const extension = vscode.extensions.getExtension(EXTENSION_ID);
  assert.ok(extension, 'development extension was not discovered');

  const originalText = document.getText();
  try {
    await extension.activate();
    await requestExpectedHover(document);
    await eventually(
      () => vscode.languages.getDiagnostics(document.uri).length === 0,
      'valid fixture retained diagnostics'
    );

    await replaceDocument(document, originalText + '\nconst broken: missing;\n');
    await eventually(
      () => vscode.languages.getDiagnostics(document.uri).length > 0,
      'invalid edit did not publish diagnostics'
    );

    await replaceDocument(document, originalText);
    await eventually(
      () => vscode.languages.getDiagnostics(document.uri).length === 0,
      'diagnostics did not clear after restoring valid source'
    );

    await vscode.commands.executeCommand('kinmokusei.restartLanguageServer');
    await requestExpectedHover(document);
  } finally {
    await replaceDocument(document, originalText);
    await configuration.update(
      'server.path',
      previousServerPath,
      vscode.ConfigurationTarget.Workspace
    );
  }
}

module.exports = { run };
