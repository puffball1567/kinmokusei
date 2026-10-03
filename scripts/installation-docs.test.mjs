import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const guide = fs.readFileSync(new URL('../website/learn/installation.md', import.meta.url), 'utf8');

for (const [platform, tool] of [['Linux', 'sha256sum'], ['macOS', 'shasum']]) {
  const available = spawnSync(tool, ['--version'], { encoding: 'utf8' }).status === 0;
  test(`${platform} installation checksum commands verify only the selected archive`, {
    skip: available ? false : `${tool} is not available on this host`,
  }, t => {
    const block = guide.match(new RegExp('```sh \\[' + platform + '\\]\\n([\\s\\S]*?)```'));
    assert.ok(block, 'missing documented installation commands');
    const lines = block[1].trim().split('\n');
    const extraction = lines.findIndex(line => line.startsWith('tar '));
    assert.ok(extraction > 0, 'checksum commands must precede extraction');
    const commands = lines.slice(0, extraction).join('\n');
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'keika-install-docs-'));
    t.after(() => fs.rmSync(root, { recursive: true, force: true }));
    const archiveName = spawnSync('sh', ['-c', commands.split('\n').slice(0, 2).join('\n') + '\nprintf "%s" "$release_archive"'], {
      cwd: root, encoding: 'utf8',
    });
    assert.equal(archiveName.status, 0, archiveName.stderr);
    assert.match(archiveName.stdout, /^keika_[0-9.]+_(linux|darwin)_(amd64|arm64)\.tar\.gz$/);
    const archive = path.join(root, archiveName.stdout);
    const content = 'documentation checksum fixture\n';
    const checksum = createHash('sha256').update(content).digest('hex');
    const sums = path.join(root, 'SHA256SUMS');
    fs.writeFileSync(archive, content);
    // Other release assets are intentionally absent, just as for a normal install.
    fs.writeFileSync(sums, `${checksum}  ${archiveName.stdout}\n${checksum}  another-platform.zip\n${checksum}  editor.vsix\n`);
    const check = () => spawnSync('sh', ['-c', commands], { cwd: root, encoding: 'utf8' });
    assert.equal(check().status, 0, 'one valid archive should not require other release assets');
    fs.writeFileSync(archive, 'tampered archive\n');
    assert.notEqual(check().status, 0, 'tampered content must fail');
    fs.writeFileSync(archive, content);
    fs.writeFileSync(sums, `${checksum}  another-platform.zip\n`);
    assert.notEqual(check().status, 0, 'a missing selected checksum must fail');
    fs.writeFileSync(sums, '');
    assert.notEqual(check().status, 0, 'empty checksum input must fail');
  });
}
