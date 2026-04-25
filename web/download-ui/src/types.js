/**
 * TenzoShare Public Download API — type contract.
 *
 * These types describe every field returned by the public transfer endpoints.
 * Anyone replacing this UI only needs to implement calls to these two endpoints:
 *
 *   GET /api/v1/t/:slug[?password=...]
 *   GET /api/v1/t/:slug/files/:fileId/download[?password=...]
 *
 * No authentication token is required.
 */
export {};
