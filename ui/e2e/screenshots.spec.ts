import { test } from '@playwright/test';

// Mock data that represents a realistic UI state.
const MOCK_REPO = {
	id: 'repo_mock01',
	owner: 'acme',
	name: 'webapp',
	full_name: 'acme/webapp',
	created_at: '2025-01-15T10:00:00Z'
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
		logs: ['[agent] Analyzing codebase...', '[agent] Found connection pool config'],
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
		logs: ['[agent] Implementation complete'],
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
		logs: ['[agent] Done'],
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
		logs: ['[agent] Error: incompatible schema'],
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
		logs: ['[agent] Re-analyzing test failures...', '[agent] Applying fix to connection teardown'],
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
		logs: ['[agent] Previous attempt completed with errors'],
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
	await page.route('**/api/v1/tasks/*/logs', (route) => {
		return route.fulfill({
			status: 200,
			headers: { 'Content-Type': 'text/event-stream' },
			body: 'event: logs\ndata: []\n\n'
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

	test('create task dialog', async ({ page }, testInfo) => {
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

		await page.screenshot({
			path: `screenshots/create-task-dialog-${testInfo.project.name}.png`,
			fullPage: true
		});
	});
});
