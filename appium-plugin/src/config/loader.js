import path from 'node:path';
import fs from 'node:fs';

/**
 * loadConfig
 *
 * Reads the plugin configuration from either:
 *  1. An inline JSON string
 *  2. A file path pointing to a JSON config file
 *
 * @param {string|undefined} input  Inline JSON or path to JSON file
 * @returns {object}  Parsed configuration object
 * @throws {Error}  When no config is provided, or parsing/reading fails
 */
export function loadConfig(input) {
    if (!input) {
        throw new Error(
            'GADS: config is required – supply --plugin-gads-config with an in-line json or path to json file'
        );
    }

    // Normalize and trim the input string
    const txt = String(input).trim()

    // Case 1: Inline JSON (starts with '{')
    if (txt.startsWith('{')) {
        try {
            // Parse and return the JSON
            return JSON.parse(txt)
        } catch (err) {
            throw new Error(`GADS: failed to parse inline JSON config – ${err.message}`)
        }
    }

    // Case 2: File path to a JSON file
    const filePath = path.resolve(txt);
    if (!fs.existsSync(filePath)) {
        throw new Error(`GADS: config file not found at ${filePath}`)
    }
    try {
        // Read file contents and parse as JSON
        const fileContents = fs.readFileSync(filePath, 'utf8')
        return JSON.parse(fileContents)
    } catch (err) {
        throw new Error(`GADS: failed to parse config file at ${filePath} – ${err.message}`)
    }
}