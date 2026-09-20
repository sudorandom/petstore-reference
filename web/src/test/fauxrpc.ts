import { spawn, ChildProcess } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export interface FauxRPCServer {
  url: string;
  port: number;
  stop: () => Promise<void>;
}

/**
 * Finds an available TCP port on localhost.
 */
export async function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.listen(0, '127.0.0.1', () => {
      const addr = srv.address();
      if (!addr || typeof addr === 'string') {
        srv.close(() => reject(new Error('Failed to obtain free port')));
        return;
      }
      const port = addr.port;
      srv.close(() => resolve(port));
    });
    srv.on('error', reject);
  });
}

/**
 * Spawns an ephemeral FauxRPC mock server pointing to the protobuf descriptor image.
 * If FAUXRPC_URL is set in environment, uses that existing instance instead of spawning a new one.
 */
export async function startFauxRPC(stubsSubdir: string = 'stubs/normal'): Promise<FauxRPCServer> {
  if (process.env.FAUXRPC_URL) {
    return {
      url: process.env.FAUXRPC_URL,
      port: 0,
      stop: async () => {},
    };
  }

  const port = await getFreePort();
  const schemaPath = path.resolve(__dirname, '../../../gen/image.binpb');
  const stubsPath = path.resolve(__dirname, '../../../', stubsSubdir);

  const homeDir = process.env.HOME || '';
  const miseFauxRPC = path.join(homeDir, '.local/share/mise/shims/fauxrpc');
  const fauxrpcBin = process.env.FAUXRPC_BIN || (fs.existsSync(miseFauxRPC) ? miseFauxRPC : 'fauxrpc');

  const proc: ChildProcess = spawn(
    fauxrpcBin,
    ['run', `--schema=${schemaPath}`, `--stubs=${stubsPath}`, '--static-seed', `--addr=127.0.0.1:${port}`],
    { stdio: ['ignore', 'pipe', 'pipe'] }
  );

  const url = `http://127.0.0.1:${port}`;

  // Poll until the FauxRPC documentation / server is ready
  const startTime = Date.now();
  let ready = false;
  while (Date.now() - startTime < 8000) {
    try {
      const res = await fetch(`${url}/fauxrpc/docs/`);
      if (res.ok) {
        ready = true;
        break;
      }
    } catch {
      // Server still starting up, wait briefly
    }
    await new Promise((r) => setTimeout(r, 100));
  }

  if (!ready) {
    proc.kill();
    throw new Error(`FauxRPC server failed to start on ${url} within 8s`);
  }

  return {
    url,
    port,
    stop: async () => {
      proc.kill('SIGTERM');
    },
  };
}
