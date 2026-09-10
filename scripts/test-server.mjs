// Isolated browser-test server. No engine, personal rooms or system changes.
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';

const port = process.argv[2];
if (!/^\d{5}$/.test(port || '')) throw new Error('Expected explicit test port');
const directory = await mkdtemp(path.join(tmpdir(), 'elciber-ui-'));
const executable = path.resolve('dist', process.platform === 'win32' ? 'elciber.exe' : 'elciber');
const child = spawn(executable, ['--listen', `127.0.0.1:${port}`, '--data-dir', path.join(directory, 'data'), '--engine-dir', path.join(directory, 'no-engine'), '--no-browser'], {
  stdio: ['ignore', 'pipe', 'pipe'],
  env: Object.fromEntries(Object.entries(process.env).filter(([key]) => ['PATH', 'HOME', 'USERPROFILE', 'SystemRoot', 'SYSTEMROOT', 'TEMP', 'TMP', 'LOCALAPPDATA', 'APPDATA'].includes(key))),
});
child.stdout.pipe(process.stdout);
child.stderr.pipe(process.stderr);
let stopping = false;
function stop() { if (!stopping) { stopping = true; child.kill('SIGTERM'); } }
process.on('SIGTERM', stop);
process.on('SIGINT', stop);
child.on('error', async () => { await rm(directory, { recursive: true, force: true }); process.exit(1); });
child.on('exit', async (code) => { await rm(directory, { recursive: true, force: true }); process.exit(stopping ? 0 : (code ?? 1)); });
