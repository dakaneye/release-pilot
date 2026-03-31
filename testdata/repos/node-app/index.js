/**
 * Parse a user agent string into its components.
 * @param {string} ua - The user agent string.
 * @returns {{ browser: string, version: string }}
 */
export function parseUserAgent(ua) {
  const match = ua.match(/(Chrome|Firefox|Safari|Edge)\/(\d+)/);
  if (!match) return { browser: "Unknown", version: "0" };
  return { browser: match[1], version: match[2] };
}

/**
 * Format bytes into a human-readable string.
 * @param {number} bytes
 * @returns {string}
 */
export function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
}
