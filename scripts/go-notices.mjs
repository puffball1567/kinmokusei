// Release-only collector. No network access or third-party Node dependencies.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const licenseName = /^(LICENSE|LICENCE|COPYING|NOTICE|PATENTS)([._-].*)?$/i;
const sourceFields = ['GoFiles', 'CgoFiles', 'CFiles', 'CXXFiles', 'MFiles', 'HFiles', 'FFiles', 'SFiles', 'SwigFiles', 'SwigCXXFiles', 'EmbedFiles'];

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
  for (const pkg of packages) {
    if (pkg.Module?.Main) continue;
    if (!pkg.Standard) throw new Error(`dependency requires license review: ${pkg.ImportPath}`);
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
  const index = `Go third-party attribution materials\nToolchain: ${version}\nTarget: ${target}\n\nOriginal files are copied verbatim. Source supplements may include code\nnot retained by the linker. See THIRD_PARTY_NOTICES.md.\n\nPackages:\n${[...new Set(imports)].sort().join('\n')}\n\nFiles:\n${entries.join('\n')}\n`;
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
    const packages = run(['list', '-deps', '-json', './cmd/keika']).trim().split(/\n(?=\{)/).map(JSON.parse);
    const files = collect(environment.GOROOT, packages, environment.GOVERSION, `${environment.GOOS}/${environment.GOARCH}`);
    materialize(files, destination, mode === '--check');
    console.log(`${mode === '--check' ? 'verified' : 'collected'} ${files.size} Go attribution files for ${environment.GOOS}/${environment.GOARCH}`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
