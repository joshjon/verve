import { test } from '@playwright/test';

// Mock data that represents a realistic UI state.
const MOCK_REPO = {
	id: 'repo_mock01',
	owner: 'acme',
	name: 'webapp',
	full_name: 'acme/webapp',
	created_at: '2025-01-15T10:00:00Z'
};

// Sample agent logs that showcase all the different log types and syntax highlighting.
// These are used by the running/review task detail screenshots so we can preview how
// the terminal rendering looks for each prefix and inline formatting rule.
const SAMPLE_LOGS_RUNNING: Record<number, string[]> = {
	1: [
		'[system] Task tsk_running01 claimed by worker w_abc123',
		'[system] Spinning up agent container verve-agent:latest',
		'[system] Container started in 2.4s — attached to stdout/stderr',
		'',
		'[agent] Analyzing codebase structure...',
		'[agent] Reading config at src/db/pool.ts',
		'[agent] Found 3 files related to connection pooling',
		'',
		'[claude] I can see the connection pool is configured in src/db/pool.ts with a max of 5 connections.',
		'[claude] The issue is that each request creates a new pool instead of reusing the shared instance.',
		'[claude] Let me trace through the request lifecycle to understand the full flow:',
		'[claude]   1. src/server.ts creates the Express app',
		'[claude]   2. src/middleware/db.ts attaches a pool to each request',
		'[claude]   3. src/db/pool.ts exports createPool() which is called per-request',
		'[claude] The fix is to call createPool() once at startup and share it via middleware.',
		'',
		'[agent] Editing src/db/pool.ts',
		'$ git diff --stat',
		' src/db/pool.ts       | 18 +++++++++-------',
		' src/middleware/db.ts  |  4 ++--',
		' 2 files changed, 12 insertions(+), 10 deletions(-)',
		'',
		'[claude] I\'ve refactored the pool to be a singleton. Now testing the change:',
		'',
		'[agent] Running test suite',
		'$ npm test',
		'  PASS src/db/pool.test.ts (3 tests, 0.8s)',
		'  PASS src/middleware/db.test.ts (5 tests, 1.2s)',
		'  PASS src/routes/users.test.ts (12 tests, 2.1s)',
		'',
		'[info] All 20 tests passed in 4.1s',
		'[claude] Tests are passing. The connection pool is now properly shared across requests.',
		'[claude] Under load this should reduce connection count from O(n) to a fixed pool of 5.',
		'',
		'[agent] Creating commit "fix: share database connection pool across requests"',
		'[agent] Pushing to branch verve/fix-db-pool',
		'[system] Agent completed successfully'
	]
};

const SAMPLE_LOGS_REVIEW: Record<number, string[]> = {
	1: [
		'[system] Task tsk_review01 claimed by worker w_xyz789',
		'[system] Container verve-agent:latest ready',
		'',
		'[agent] Starting work on dark mode support',
		'[agent] Scanning project for existing theme infrastructure',
		'',
		'[claude] The project uses Tailwind CSS with a custom color palette defined in tailwind.config.js.',
		'[claude] There\'s no existing dark mode setup. I\'ll implement this using Tailwind\'s "class" strategy',
		'[claude] so we can toggle dark mode programmatically and persist the user\'s preference.',
		'',
		'[agent] Creating src/lib/stores/theme.ts',
		'[agent] Updating src/app.html to add class to <html> element',
		'[agent] Modifying src/lib/components/Header.svelte — adding toggle button',
		'',
		'[claude] I need to handle 3 preference states:',
		'[claude]   - "light" — always light',
		'[claude]   - "dark" — always dark',
		'[claude]   - "system" — follow OS preference via matchMedia("(prefers-color-scheme: dark)")',
		'[claude] Storing the choice in localStorage under key "theme-preference".',
		'',
		'[agent] Writing theme store implementation',
		'$ cat src/lib/stores/theme.ts',
		'  export type ThemePreference = "light" | "dark" | "system";',
		'  export const theme = writable<ThemePreference>("system");',
		'',
		'[warn] tailwind.config.js: darkMode is set to "media" — changing to "class"',
		'[agent] Updated tailwind.config.js',
		'',
		'[claude] Now adding the toggle component. Using a sun/moon icon pair with smooth transition.',
		'',
		'[agent] Running build to verify no errors',
		'$ npm run build',
		'  vite v5.2.0 building for production...',
		'  143 modules transformed',
		'  dist/index.html   0.45 kB gzip',
		'  dist/assets/app.js   82.3 kB gzip',
		'  Build completed in 3.8s',
		'',
		'[info] Build succeeded — 0 errors, 0 warnings',
		'[agent] Running tests',
		'$ npm test',
		'  PASS src/lib/stores/theme.test.ts (4 tests)',
		'  PASS src/lib/components/Header.test.ts (6 tests)',
		'',
		'[claude] Everything looks good. The theme toggle is working correctly with all 3 modes.',
		'[claude] Dark mode colors are applied via Tailwind\'s dark: prefix throughout the app.',
		'',
		'[agent] Creating commit and pushing to verve/add-dark-mode',
		'[agent] Creating pull request #42: "Add dark mode support with system preference detection"',
		'[info] PR created: https://github.com/acme/webapp/pull/42',
		'[system] Agent completed — PR ready for review'
	]
};

const SAMPLE_LOGS_RETRY_RUNNING: Record<number, string[]> = {
	1: [
		'[system] Task tsk_retry_running01 claimed by worker w_def456',
		'[agent] Analyzing flaky test failures',
		'[claude] The integration tests are failing intermittently due to database connection teardown race conditions.',
		'[agent] Attempting fix in src/tests/helpers/db-setup.ts',
		'[error] Test suite failed: 3 of 15 tests timed out',
		'[agent] Fix did not resolve all failures',
		'[system] Agent completed with failures'
	],
	2: [
		'[system] Retry attempt 2 — task tsk_retry_running01 claimed by worker w_def456',
		'[system] Container verve-agent:latest ready',
		'',
		'[agent] Re-analyzing test failures from attempt 1',
		'[agent] Reading CI logs from previous run',
		'',
		'[claude] Looking at the previous failure more carefully, the root cause is not just the teardown.',
		'[claude] The tests share a single database instance and run in parallel, causing lock contention.',
		'[claude] I need to:',
		'[claude]   1. Give each test file its own isolated database',
		'[claude]   2. Fix the connection teardown to use proper async cleanup',
		'[claude]   3. Add a retry wrapper for transient connection errors',
		'',
		'[agent] Editing src/tests/helpers/db-setup.ts',
		'[agent] Editing src/tests/integration/users.test.ts',
		'[agent] Editing src/tests/integration/orders.test.ts',
		'',
		'[claude] Each test file now gets a unique database via template databases. This eliminates',
		'[claude] the parallel execution conflicts entirely.',
		'',
		'$ npm run test:integration',
		'  PASS src/tests/integration/users.test.ts (8 tests, 3.2s)',
		'  PASS src/tests/integration/orders.test.ts (5 tests, 2.8s)',
		'  PASS src/tests/integration/auth.test.ts (2 tests, 1.1s)',
		'',
		'[info] All 15 integration tests passed — 0 flakes across 3 consecutive runs',
		'[agent] Pushing changes to verve/fix-flaky-tests',
		'[system] Agent completed successfully — PR #45 updated'
	]
};

const SAMPLE_LOGS_FAILED: Record<number, string[]> = {
	1: [
		'[system] Task tsk_failed01 claimed by worker w_ghi789',
		'[agent] Starting ORM migration from raw SQL to Drizzle',
		'[agent] Found 24 query files in src/db/queries/',
		'[claude] This is a large migration. I\'ll work through the queries module by module.',
		'[agent] Migrating src/db/queries/users.ts',
		'[agent] Migrating src/db/queries/orders.ts',
		'[error] Schema incompatibility: orders table uses a composite key that Drizzle doesn\'t support natively',
		'[claude] The orders table has a composite primary key (user_id, order_id) which requires a workaround in Drizzle.',
		'[claude] I\'ll use the composite primaryKey helper from drizzle-orm/pg-core.',
		'[agent] Applied workaround for composite keys',
		'$ npm test',
		'  FAIL src/db/queries/orders.test.ts',
		'    TypeError: Cannot read properties of undefined (reading \'id\')',
		'  FAIL src/db/queries/users.test.ts',
		'    Error: relation "users_new" does not exist',
		'[error] 8 of 24 tests failed — migration incomplete',
		'[system] Agent completed with failures'
	],
	2: [
		'[system] Retry attempt 2 — task tsk_failed01 claimed by worker w_ghi789',
		'[agent] Resuming ORM migration — addressing 8 test failures',
		'[claude] The previous attempt had 2 issues:',
		'[claude]   1. The composite key workaround was incorrect — need to use sql`` template',
		'[claude]   2. Migration created "users_new" table instead of replacing "users"',
		'[agent] Fixing schema definitions',
		'[error] Drizzle introspection failed: incompatible schema version 0.28 (expected >= 0.30)',
		'[error] Cannot proceed — Drizzle ORM version in package.json is outdated',
		'[system] Agent failed — incompatible schema version'
	]
};

const MOCK_TASKS = [
	{
		id: 'tsk_pending01',
		repo_id: 'repo_mock01',
		title: 'Add user authentication',
		description: 'Implement JWT-based auth with login/signup pages',
		status: 'pending',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Login page works', 'JWT tokens issued'],
		consecutive_failures: 0,
		cost_usd: 0,
		skip_pr: false,
		ready: true,
		created_at: '2025-06-01T09:00:00Z',
		updated_at: '2025-06-01T09:00:00Z'
	},
	{
		id: 'tsk_notready01',
		repo_id: 'repo_mock01',
		title: 'Refactor payment processing module',
		description: 'Break up the monolithic payment handler into smaller services',
		status: 'pending',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Payment tests pass', 'No regressions'],
		consecutive_failures: 0,
		cost_usd: 0,
		skip_pr: false,
		ready: false,
		created_at: '2025-06-01T08:00:00Z',
		updated_at: '2025-06-01T08:00:00Z'
	},
	{
		id: 'tsk_running01',
		repo_id: 'repo_mock01',
		title: 'Fix database connection pooling',
		description: 'Connection pool exhaustion under load',
		status: 'running',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: [],
		consecutive_failures: 0,
		cost_usd: 0.12,
		skip_pr: false,
		started_at: '2025-06-01T10:30:00Z',
		created_at: '2025-06-01T10:00:00Z',
		updated_at: '2025-06-01T10:30:00Z'
	},
	{
		id: 'tsk_review01',
		repo_id: 'repo_mock01',
		title: 'Add dark mode support',
		description: 'Implement theme toggle with system preference detection',
		status: 'review',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/42',
		pr_number: 42,
		branch_name: 'verve/add-dark-mode',
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Theme toggle in header', 'Persists preference'],
		consecutive_failures: 0,
		cost_usd: 0.45,
		skip_pr: false,
		started_at: '2025-06-01T08:00:00Z',
		duration_ms: 180000,
		created_at: '2025-06-01T07:00:00Z',
		updated_at: '2025-06-01T08:03:00Z'
	},
	{
		id: 'tsk_merged01',
		repo_id: 'repo_mock01',
		title: 'Update API documentation',
		description: 'Auto-generate OpenAPI spec from route handlers',
		status: 'merged',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/38',
		pr_number: 38,
		branch_name: 'verve/update-api-docs',
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: [],
		consecutive_failures: 0,
		cost_usd: 0.30,
		skip_pr: false,
		started_at: '2025-05-30T14:00:00Z',
		duration_ms: 120000,
		created_at: '2025-05-30T13:00:00Z',
		updated_at: '2025-05-30T14:02:00Z'
	},
	{
		id: 'tsk_failed01',
		repo_id: 'repo_mock01',
		title: 'Migrate to new ORM',
		description: 'Replace raw SQL with Drizzle ORM',
		status: 'failed',
		logs: [],
		attempt: 2,
		max_attempts: 3,
		acceptance_criteria: ['All queries migrated', 'Tests pass'],
		consecutive_failures: 1,
		cost_usd: 0.85,
		skip_pr: false,
		started_at: '2025-05-29T16:00:00Z',
		duration_ms: 300000,
		created_at: '2025-05-29T15:00:00Z',
		updated_at: '2025-05-29T16:05:00Z'
	},
	// Retry scenario: agent is actively running a retry on a task that already has a PR
	{
		id: 'tsk_retry_running01',
		repo_id: 'repo_mock01',
		title: 'Fix flaky integration tests',
		description: 'Stabilize integration test suite that randomly fails in CI',
		status: 'running',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/45',
		pr_number: 45,
		branch_name: 'verve/fix-flaky-tests',
		attempt: 2,
		max_attempts: 3,
		acceptance_criteria: ['All integration tests pass consistently', 'No test timeouts'],
		retry_reason: 'CI checks failed — test suite still flaky after first attempt',
		consecutive_failures: 1,
		cost_usd: 0.62,
		skip_pr: false,
		started_at: '2025-06-01T11:00:00Z',
		created_at: '2025-06-01T09:00:00Z',
		updated_at: '2025-06-01T11:05:00Z'
	},
	// Retry scenario: task is pending (waiting for agent pickup) with existing PR
	{
		id: 'tsk_retry_pending01',
		repo_id: 'repo_mock01',
		title: 'Improve error handling in API',
		description: 'Add proper error responses and validation to all API endpoints',
		status: 'pending',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/47',
		pr_number: 47,
		branch_name: 'verve/improve-error-handling',
		attempt: 3,
		max_attempts: 3,
		acceptance_criteria: ['All endpoints return proper error codes', 'Input validation on all routes'],
		retry_reason: 'Missing validation on POST /users endpoint',
		retry_context: 'FAIL: TestPostUsers_InvalidInput\n  Expected status 400, got 500\n  Error: missing required field validation',
		consecutive_failures: 2,
		cost_usd: 1.20,
		skip_pr: false,
		created_at: '2025-05-31T14:00:00Z',
		updated_at: '2025-06-01T12:00:00Z'
	}
];

// Map of task ID to per-attempt logs for the SSE mock.
const MOCK_TASK_LOGS: Record<string, Record<number, string[]>> = {
	tsk_running01: SAMPLE_LOGS_RUNNING,
	tsk_review01: SAMPLE_LOGS_REVIEW,
	tsk_retry_running01: SAMPLE_LOGS_RETRY_RUNNING,
	tsk_failed01: SAMPLE_LOGS_FAILED
};

// Intercept all API calls so the UI renders with mock data instead of hitting a real server.
// Routes are registered most-specific first because Playwright matches in FIFO order.
async function setupMockAPI(page: import('@playwright/test').Page) {
	// GitHub token status - report as configured so the UI shows the dashboard.
	await page.route('**/api/v1/settings/github-token', (route) =>
		route.fulfill({ json: { configured: true, fine_grained: true } })
	);

	// Default model
	await page.route('**/api/v1/settings/default-model', (route) =>
		route.fulfill({ json: { model: 'claude-sonnet-4-20250514' } })
	);

	// Repos list
	await page.route('**/api/v1/repos', (route) => {
		if (route.request().method() === 'GET') {
			return route.fulfill({ json: [MOCK_REPO] });
		}
		return route.fulfill({ json: MOCK_REPO });
	});

	// SSE events endpoint - send an init event with mock tasks then keep connection open.
	await page.route('**/api/v1/events**', (route) => {
		const body = `event: init\ndata: ${JSON.stringify(MOCK_TASKS)}\n\n`;
		return route.fulfill({
			status: 200,
			headers: {
				'Content-Type': 'text/event-stream',
				'Cache-Control': 'no-cache',
				Connection: 'keep-alive'
			},
			body
		});
	});

	// Task checks (must be before generic /tasks/* route).
	await page.route('**/api/v1/tasks/*/checks', (route) =>
		route.fulfill({
			json: {
				status: 'success',
				summary: '3/3 checks passed',
				checks: [
					{
						name: 'build',
						status: 'completed',
						conclusion: 'success',
						url: 'https://github.com'
					},
					{
						name: 'lint',
						status: 'completed',
						conclusion: 'success',
						url: 'https://github.com'
					},
					{
						name: 'test',
						status: 'completed',
						conclusion: 'success',
						url: 'https://github.com'
					}
				]
			}
		})
	);

	// Task logs SSE (must be before generic /tasks/* route).
	// Sends per-attempt logs_appended events followed by logs_done so the UI
	// renders them in the terminal with full syntax highlighting.
	await page.route('**/api/v1/tasks/*/logs', (route) => {
		const url = route.request().url();
		const taskId = url.split('/tasks/')[1]?.split('/')[0];
		const logsByAttempt = taskId ? MOCK_TASK_LOGS[taskId] : undefined;

		let body = '';
		if (logsByAttempt) {
			for (const [attempt, lines] of Object.entries(logsByAttempt)) {
				body += `event: logs_appended\ndata: ${JSON.stringify({ attempt: Number(attempt), logs: lines })}\n\n`;
			}
		}
		body += 'event: logs_done\ndata: {}\n\n';

		return route.fulfill({
			status: 200,
			headers: {
				'Content-Type': 'text/event-stream',
				'Cache-Control': 'no-cache',
				Connection: 'keep-alive'
			},
			body
		});
	});

	// Individual task detail (generic catch-all for /tasks/*).
	await page.route('**/api/v1/tasks/*', (route) => {
		const url = route.request().url();
		const taskId = url.split('/tasks/')[1]?.split('/')[0]?.split('?')[0];
		const task = MOCK_TASKS.find((t) => t.id === taskId);
		if (task) {
			return route.fulfill({ json: task });
		}
		return route.fulfill({ status: 404, json: { error: 'not found' } });
	});
}

test.describe('UI Screenshots', () => {
	test('dashboard', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/');

		// Wait for tasks to render.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {
			// Fallback: wait for any content to load.
		});

		// Give the UI a moment to settle after SSE data loads.
		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/dashboard-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - review', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_review01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-detail-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - running', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_running01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-running-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - retry running', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_retry_running01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-retry-running-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - retry pending', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_retry_pending01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-retry-pending-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - not ready', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_notready01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-not-ready-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('edit task dialog', async ({ page }, testInfo) => {
		// Use a tall viewport so the dialog's max-h-[90vh] doesn't clip content.
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page);
		await page.goto('/tasks/tsk_pending01');

		// Wait for task detail to load.
		await page.waitForTimeout(2000);

		// Click the "Edit" button to open the dialog.
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly to capture its full content.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/edit-task-dialog-${testInfo.project.name}.png`
		});
	});

	test('create task dialog', async ({ page }, testInfo) => {
		// Use a tall viewport so the dialog's max-h-[90vh] doesn't clip content.
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page);
		await page.goto('/');

		// Wait for dashboard to load.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {});
		await page.waitForTimeout(1000);

		// Click the "New Task" button to open the dialog.
		const createButton = page.getByRole('button', { name: /new task/i });
		await createButton.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly to capture its full content.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/create-task-dialog-${testInfo.project.name}.png`
		});
	});
});
