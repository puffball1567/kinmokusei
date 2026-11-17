// Release-only collector. No network access or third-party Node dependencies.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const licenseName = /^(LICENSE|LICENCE|COPYING|NOTICE|PATENTS)([._-].*)?$/i;
const sourceFields = ['GoFiles', 'CgoFiles', 'CFiles', 'CXXFiles', 'MFiles', 'HFiles', 'FFiles', 'SFiles', 'SwigFiles', 'SwigCXXFiles', 'EmbedFiles'];

// Reviewed pure-Go dependencies only. A version, package set or notice change
// fails closed and requires updating the review and retained original texts.
const reviewedModules = {
  'golang.org/x/mod@v0.23.0': {
    packages: ['golang.org/x/mod/module', 'golang.org/x/mod/semver', 'golang.org/x/mod/internal/lazyregexp'],
    notices: {
      LICENSE: '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad',
      PATENTS: '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc',
    },
  },
};

function collectReviewedModule(pkg, files) {
  const identity = `${pkg.Module?.Path}@${pkg.Module?.Version}`;
  const review = reviewedModules[identity];
  if (!review || pkg.Module.Replace || !review.packages.includes(pkg.ImportPath)) throw new Error(`dependency requires license review: ${pkg.ImportPath}`);
  if (pkg.SysoFiles?.length || sourceFields.some(field => field !== 'GoFiles' && pkg[field]?.length)) throw new Error(`native dependency requires license review: ${pkg.ImportPath}`);
  const root = fs.realpathSync(pkg.Module.Dir);
  const prefix = `licenses/modules/${identity}`;
  function add(file, suffix = '', expectedHash) {
    const relative = path.relative(root, file);
    const resolved = path.relative(root, fs.realpathSync(file));
    if (!relative || relative.startsWith('..') || path.isAbsolute(relative) || !resolved || resolved.startsWith('..') || path.isAbsolute(resolved)) throw new Error('module notice path outside reviewed module');
    if (!fs.lstatSync(file).isFile()) throw new Error('module notice is not a regular file');
    const data = fs.readFileSync(file);
    if (!data.length) throw new Error('empty module notice');
    if (expectedHash && createHash('sha256').update(data).digest('hex') !== expectedHash) throw new Error(`reviewed module notice changed: ${identity}/${relative}`);
    files.set(`${prefix}/${relative.split(path.sep).join('/')}${suffix}`, data);
  }
  for (const [name, hash] of Object.entries(review.notices)) add(path.join(root, name), '', hash);
  // Full selected sources retain every per-file copyright and disclaimer,
  // including any extra notice the upstream files might contain.
  for (const name of pkg.GoFiles ?? []) add(path.join(pkg.Dir, name), '.txt');
  return identity;
}

export function collect(goroot, packages, version, target) {
  const files = new Map();
  function relative(file) {
    const name = path.relative(goroot, file);
    if (!name || name.startsWith('..') || path.isAbsolute(name)) throw new Error('notice path outside Go toolchain');
    return name.split(path.sep).join('/');
  }
  function add(file, suffix = '') {
    const name = relative(file);
    const stat = fs.lstatSync(file);
    if (!stat.isFile()) throw new Error(`notice is not a regular file: ${name}`);
    const data = fs.readFileSync(file);
    if (!data.length) throw new Error(`empty notice: ${name}`);
    files.set(`licenses/go/${name}${suffix}`, data);
  }
  add(path.join(goroot, 'LICENSE'));
  add(path.join(goroot, 'PATENTS'));
  const scanned = new Set();
  const imports = [];
  const modules = new Set();
  for (const pkg of packages) {
    if (pkg.Module?.Main) continue;
    if (!pkg.Standard) {
      modules.add(collectReviewedModule(pkg, files));
      imports.push(pkg.ImportPath);
      continue;
    }
    if (!pkg.Dir) continue; // pseudo packages such as unsafe
    relative(pkg.Dir);
    imports.push(pkg.ImportPath);
    for (let dir = pkg.Dir; dir !== goroot; dir = path.dirname(dir)) {
      if (scanned.has(dir)) break;
      scanned.add(dir);
      for (const name of fs.readdirSync(dir).sort()) {
        if (licenseName.test(name)) add(path.join(dir, name));
      }
    }
    // Keep complete files, not guessed excerpts: additional notices can occur
    // after the package clause or alongside assembly and generated tables.
    for (const field of sourceFields) {
      for (const name of pkg[field] ?? []) {
        const file = path.join(pkg.Dir, name);
        relative(file);
        const text = fs.readFileSync(file, 'utf8');
        const withoutGoHeader = text.replace(/\/\/ Copyright[^\n]*The Go Authors[^\n]*\r?\n\/\/ Use of this source code is governed by a BSD-style\r?\n\/\/ license that can be found in the LICENSE file\.[^\n]*/g, '');
        if (/copyright|licen[cs]e|permission|redistribution|public domain/i.test(withoutGoHeader)) add(file, '.txt');
      }
    }
    if (pkg.SysoFiles?.length) throw new Error(`binary object requires license review: ${pkg.ImportPath}`);
  }
  const entries = [...files.keys()].sort();
  const index = `Go third-party attribution materials\nToolchain: ${version}\nTarget: ${target}\n\nOriginal files are copied verbatim. Source supplements may include code\nnot retained by the linker. See THIRD_PARTY_NOTICES.md.\n\nModules:\n${[...modules].sort().join('\n')}\n\nPackages:\n${[...new Set(imports)].sort().join('\n')}\n\nFiles:\n${entries.join('\n')}\n`;
  files.set('licenses/go/INDEX.txt', Buffer.from(index));
  return files;
}

export function materialize(files, destination, check = false) {
  for (const [name, data] of files) {
    const file = path.join(destination, name);
    if (check) {
      if (!fs.lstatSync(file).isFile() || !fs.readFileSync(file).equals(data)) throw new Error(`notice differs: ${name}`);
    } else {
      fs.mkdirSync(path.dirname(file), { recursive: true });
      fs.writeFileSync(file, data, { flag: 'wx' });
    }
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const [mode, destination] = process.argv.slice(2);
    if (!['--write', '--check'].includes(mode) || !destination || process.argv.length !== 4) throw new Error('usage: node scripts/go-notices.mjs --write|--check <staging-directory>');
    const run = args => execFileSync('go', args, { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
    const environment = JSON.parse(run(['env', '-json', 'GOROOT', 'GOVERSION', 'GOOS', 'GOARCH']));
    run(['mod', 'verify']);
    const packages = run(['list', '-deps', '-json', './cmd/keika']).trim().split(/\n(?=\{)/).map(JSON.parse);
    const files = collect(environment.GOROOT, packages, environment.GOVERSION, `${environment.GOOS}/${environment.GOARCH}`);
    materialize(files, destination, mode === '--check');
    console.log(`${mode === '--check' ? 'verified' : 'collected'} ${files.size} Go attribution files for ${environment.GOOS}/${environment.GOARCH}`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
