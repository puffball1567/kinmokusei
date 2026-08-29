'use strict';

const { createController } = require('./client');

let controller;

async function activate(context) {
  const vscode = require('vscode');
  const { LanguageClient } = require('vscode-languageclient/node');
  controller = createController({ vscode, LanguageClient });
  await controller.activate(context);
}

async function deactivate() {
  const active = controller;
  controller = undefined;
  if (active) {
    await active.deactivate();
  }
}

module.exports = { activate, deactivate };
