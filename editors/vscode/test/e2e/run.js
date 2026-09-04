'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { runTests } = require('@vscode/test-electron');

const extensionDevelopmentPath = path.resolve(__dirname, '..', '..');
const repositoryRoot = path.resolve(extensionDevelopmentPath, '..', '..');
const extensionTestsPath = path.resolve(__dirname, 'suite');
const fixturePath = path.resolve(__dirname, 'fixture');

function localVSCodeExecutable() {
  if (process.env.VSCODE_EXECUTABLE_PATH) {
    return process.env.VSCODE_EXECUTABLE_PATH;
  }
  const candidates = process.platform === 'linux'
    ? ['/usr/share/code/code', '/usr/bin/code']
    : [];
  return candidates.find((candidate) => fs.existsSync(candidate));
}

async function main() {
  const temporaryDirectory = fs.mkdtempSync(
    path.join(os.tmpdir(), 'kinmokusei-vscode-e2e-')
  );
  const serverPath = path.join(
    temporaryDirectory,
    process.platform === 'win32' ? 'keika.exe' : 'keika'
  );

  try {
    const goCacheRoot = path.join(os.tmpdir(), 'kinmokusei-vscode-e2e-go');
    const goEnvironment = {
      ...process.env,
      GOPATH: path.join(goCacheRoot, 'gopath'),
      GOCACHE: path.join(goCacheRoot, 'gocache'),
      GOTMPDIR: path.join(temporaryDirectory, 'gotmp')
    };
    fs.mkdirSync(goEnvironment.GOPATH, { recursive: true });
    fs.mkdirSync(goEnvironment.GOCACHE, { recursive: true });
    fs.mkdirSync(goEnvironment.GOTMPDIR, { recursive: true });
    execFileSync('go', ['build', '-buildvcs=false', '-o', serverPath, './cmd/keika'], {
      cwd: repositoryRoot,
      env: goEnvironment,
      stdio: 'inherit'
    });

    const options = {
      extensionDevelopmentPath,
      extensionTestsPath,
      launchArgs: [
        fixturePath,
        '--disable-extensions',
        '--disable-workspace-trust',
        '--user-data-dir=' + path.join(temporaryDirectory, 'user-data'),
        '--extensions-dir=' + path.join(temporaryDirectory, 'extensions')
      ],
      extensionTestsEnv: {
        KINMOKUSEI_E2E_SERVER_PATH: serverPath
      }
    };
    const executable = localVSCodeExecutable();
    if (executable) {
      options.vscodeExecutablePath = executable;
    } else {
      options.version = '1.103.0';
    }
    await runTests(options);
  } finally {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
