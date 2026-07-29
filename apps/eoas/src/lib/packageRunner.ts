import fs from 'fs-extra';
import path from 'path';

const DEFAULT_PACKAGE_RUNNER = 'npx';

const VALID_RUNNER_RE = /^[a-zA-Z0-9._-]+$/;

function assertValidRunner(value: string, source: string): void {
  if (!VALID_RUNNER_RE.test(value)) {
    throw new Error(
      `Invalid package runner "${value}" (from ${source}). Expected a simple binary name like npx, bunx or pnpx.`
    );
  }
}

const PACKAGE_MANAGER_RUNNERS: Record<string, string> = {
  bun: 'bunx',
  pnpm: 'pnpx',
  yarn: 'npx',
  npm: 'npx',
};

/**
 * Resolves the package runner command to use for spawning Expo CLI commands.
 *
 * Priority:
 * 1. Explicit value passed as argument (e.g. from --packageRunner CLI flag)
 * 2. EOAS_PACKAGE_RUNNER environment variable
 * 3. Inferred from packageManager field in package.json
 * 4. Falls back to 'npx'
 *
 * Supported values: npx, bunx, pnpx, or any other package runner binary.
 */
export function resolvePackageRunner(explicit?: string, projectDir?: string): string {
  if (explicit) {
    assertValidRunner(explicit, '--packageRunner flag');
    return explicit;
  }
  if (process.env.EOAS_PACKAGE_RUNNER) {
    assertValidRunner(process.env.EOAS_PACKAGE_RUNNER, 'EOAS_PACKAGE_RUNNER environment variable');
    return process.env.EOAS_PACKAGE_RUNNER;
  }

  if (projectDir) {
    const detected = detectRunnerFromPackageJson(projectDir);
    if (detected) {
      return detected;
    }
  }

  return DEFAULT_PACKAGE_RUNNER;
}

/**
 * Walks up from projectDir to find a package.json with a packageManager field
 * and maps it to the corresponding package runner binary.
 */
function detectRunnerFromPackageJson(startDir: string): string | undefined {
  let dir = path.resolve(startDir);
  const root = path.parse(dir).root;

  while (dir !== root) {
    const pkgPath = path.join(dir, 'package.json');
    try {
      // 패키지 러너 해석은 spawn 직전 동기 경로에서 호출되므로 동기 fs 가
      // 의도적이다. 상위 디렉터리를 한 번 훑는 가벼운 작업이라 블로킹도 무시 가능.
      // eslint-disable-next-line node/no-sync
      if (fs.existsSync(pkgPath)) {
        // eslint-disable-next-line node/no-sync
        const pkg = fs.readJsonSync(pkgPath);
        if (pkg.packageManager) {
          const name = pkg.packageManager.split('@')[0];
          return PACKAGE_MANAGER_RUNNERS[name];
        }
      }
    } catch {
      // Ignore read errors, keep walking up
    }
    dir = path.dirname(dir);
  }

  return undefined;
}
