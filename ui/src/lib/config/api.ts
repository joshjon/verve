// API base URL - configurable via environment variable.
// When unset, defaults to localhost:7400 for local development.
// Set to empty string for same-origin requests (e.g. behind nginx proxy).
export const API_BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:7400';
