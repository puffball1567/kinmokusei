import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { collect, materialize } from './go-notices.mjs';

test('preserves complete notices, nested licenses, target identity and deterministic output', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'keika-notices-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const goroot = path.join(root, 'go');
  const dir = path.join(goroot, 'src/vendor/example/pkg');
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(goroot, 'LICENSE'), 'Go license\n');
  fs.writeFileSync(path.join(goroot, 'PATENTS'), 'Patent grant\n');
  fs.writeFileSync(path.dirname(dir) + '/LICENSE', 'Vendor license\n');
  fs.writeFileSync(path.join(dir, 'NOTICE.txt'), 'Vendor notice\n');
  const source = '// Copyright 2009 The Go Authors. All rights reserved.\n// Use of this source code is governed by a BSD-style\n// license that can be found in the LICENSE file.\npackage pkg\n';
  fs.writeFileSync(path.join(dir, 'plain.go'), source);
  const special = source + '// Copyright Example Authors\n// Permission to use, copy\n// Disclaimer in full\nfunc f() {}\n';
  fs.writeFileSync(path.join(dir, 'special.go'), special);
  fs.writeFileSync(path.join(dir, 'arch.s'), '// Public domain implementation\n');
  const packages = [{ ImportPath: 'main', Module: { Main: true } }, { ImportPath: 'unsafe', Standard: true }, { ImportPath: 'vendor/example/pkg', Dir: dir, Standard: true, GoFiles: ['plain.go', 'special.go'], SFiles: ['arch.s'] }];
  const files = collect(goroot, packages, 'go1.27.0', 'linux/amd64');
  assert.equal(files.get('licenses/go/src/vendor/example/pkg/special.go.txt').toString(), special);
  assert.ok(!files.has('licenses/go/src/vendor/example/pkg/plain.go.txt'));
  assert.equal(files.get('licenses/go/src/vendor/example/LICENSE').toString(), 'Vendor license\n');
  assert.ok(files.has('licenses/go/src/vendor/example/pkg/arch.s.txt'));
  assert.match(files.get('licenses/go/INDEX.txt').toString(), /go1.27.0\nTarget: linux\/amd64/);
  assert.ok(!files.get('licenses/go/INDEX.txt').toString().includes(root));
  assert.deepEqual(collect(goroot, packages, 'go1.27.0', 'linux/amd64'), files);
  const output = path.join(root, 'output');
  materialize(files, output);
  materialize(files, output, true);
  assert.throws(() => materialize(files, output), /EEXIST/);
  fs.writeFileSync(path.join(output, 'licenses/go/LICENSE'), 'changed');
  assert.throws(() => materialize(files, output, true), /notice differs/);
  fs.rmSync(path.join(output, 'licenses/go/LICENSE'));
  assert.throws(() => materialize(files, output, true), /ENOENT/);
  assert.throws(() => collect(goroot, [{ ImportPath: 'external' }], '', ''), /requires license review/);
  assert.throws(() => collect(goroot, [{ Standard: true, Dir: root }], '', ''), /outside/);
  assert.throws(() => collect(goroot, [{ ...packages[2], SysoFiles: ['x.syso'] }], '', ''), /binary object/);
  fs.writeFileSync(path.join(goroot, 'LICENSE'), '');
  assert.throws(() => collect(goroot, packages, '', ''), /empty notice/);
});

test('reviewed module notices and selected sources are retained, unknown changes fail closed', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'keika-module-notices-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const goroot = path.join(root, 'go');
  const moduleRoot = path.join(root, 'module');
  const dir = path.join(moduleRoot, 'semver');
  fs.mkdirSync(goroot);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(goroot, 'LICENSE'), 'Go license\n');
  fs.writeFileSync(path.join(goroot, 'PATENTS'), 'Go patents\n');
  for (const name of ['LICENSE', 'PATENTS']) {
    fs.writeFileSync(path.join(moduleRoot, name), fs.readFileSync(fileURLToPath(new URL(`../third_party/golang.org_x_mod/${name}`, import.meta.url))));
  }
  const source = '// Copyright 2018 The Go Authors. All rights reserved.\npackage semver\n';
  fs.writeFileSync(path.join(dir, 'semver.go'), source);
  const pkg = { ImportPath: 'golang.org/x/mod/semver', Dir: dir, GoFiles: ['semver.go'], Module: { Path: 'golang.org/x/mod', Version: 'v0.23.0', Dir: moduleRoot } };
  const prefix = 'licenses/modules/golang.org/x/mod@v0.23.0/';
  const files = collect(goroot, [pkg], 'go1.27.3', 'linux/amd64');
  assert.equal(files.get(`${prefix}LICENSE`).toString(), fs.readFileSync(path.join(moduleRoot, 'LICENSE'), 'utf8'));
  assert.equal(files.get(`${prefix}PATENTS`).toString(), fs.readFileSync(path.join(moduleRoot, 'PATENTS'), 'utf8'));
  assert.equal(files.get(`${prefix}semver/semver.go.txt`).toString(), source);
  assert.match(files.get('licenses/go/INDEX.txt').toString(), /golang.org\/x\/mod@v0\.23\.0/);
  assert.ok(!files.get('licenses/go/INDEX.txt').toString().includes(root));
  assert.deepEqual(collect(goroot, [pkg], 'go1.27.3', 'linux/amd64'), files);
  const output = path.join(root, 'output');
  materialize(files, output);
  materialize(files, output, true);
  fs.writeFileSync(path.join(output, prefix, 'LICENSE'), 'changed');
  assert.throws(() => materialize(files, output, true), /notice differs/);
  for (const changed of [
    { ...pkg, Module: { ...pkg.Module, Version: 'v0.24.0' } },
    { ...pkg, Module: { ...pkg.Module, Replace: { Dir: moduleRoot } } },
    { ...pkg, ImportPath: 'golang.org/x/mod/zip' },
    { ...pkg, CFiles: ['native.c'] },
    { ...pkg, SysoFiles: ['native.syso'] },
  ]) assert.throws(() => collect(goroot, [changed], '', ''), /requires license review/);
  fs.writeFileSync(path.join(root, 'outside.go'), 'package outside\n');
  assert.throws(() => collect(goroot, [{ ...pkg, GoFiles: ['../../outside.go'] }], '', ''), /outside reviewed module/);
  fs.writeFileSync(path.join(moduleRoot, 'LICENSE'), 'different license');
  assert.throws(() => collect(goroot, [pkg], '', ''), /reviewed module notice changed/);
});
