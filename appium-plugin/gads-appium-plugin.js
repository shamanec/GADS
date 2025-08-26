import { BasePlugin } from '@appium/base-plugin';
import { logger } from '@appium/support';
import path from 'node:path';
import fs from 'node:fs';
import axios from 'axios';

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
function loadConfig(input) {
    if (!input) {
        throw new Error(
            'GADS: config is required – supply --plugin-gads-config with an in-line json or path to json file'
        );
    }

    // Normalize and trim the input string
    const txt = String(input).trim();

    // Case 1: Inline JSON (starts with '{')
    if (txt.startsWith('{')) {
        try {
            // Parse and return the JSON
            return JSON.parse(txt);
        } catch (err) {
            throw new Error(`GADS: failed to parse inline JSON config – ${err.message}`);
        }
    }

    // Case 2: File path to a JSON file
    const filePath = path.resolve(txt);
    if (!fs.existsSync(filePath)) {
        throw new Error(`GADS: config file not found at ${filePath}`);
    }
    try {
        // Read file contents and parse as JSON
        const fileContents = fs.readFileSync(filePath, 'utf8');
        return JSON.parse(fileContents);
    } catch (err) {
        throw new Error(`GADS: failed to parse config file at ${filePath} – ${err.message}`);
    }
}

/**
     * This function classifies the type of action performed that we can use for session logging
     * @param {string} commandName The name of the execute command
     * @param {*} args The arguments for the command
     * @returns 
     */
function classify(commandName, args) {
    log.info(`Classifying command ${commandName}`)
    switch (commandName) {
        case 'click': return { action: 'Tap', source: 'click' };
        case 'setValue': return { action: 'Type', source: 'setValue' };
        case 'performActions': return { action: 'Gesture', source: 'performActions' };
        case 'executeScript': {
            const script = args?.[0];
            if (typeof script === 'string' && script.startsWith('mobile:')) {
                return { action: 'Mobile-command', source: script };
            }
            return null;
        }
        case 'findElement': return { action: 'Find Element', source: 'findElement' }
        case 'findElements': return { action: 'Find Elements', source: 'findElements' }
        default: return null;
    }
}

/**
 * This function redacts sensitive data from logs like when typing text
 * @param {*} payload the log payload
 * @returns 
 */
function redact(payload) {
    const KEYS = new Set(['text', 'value']);
    const seen = new WeakSet();
    const walk = (v) => {
        if (v && typeof v === 'object') {
            if (seen.has(v)) return v;
            seen.add(v);
            if (Array.isArray(v)) return v.map(walk);
            const out = {};
            for (const [k, val] of Object.entries(v)) {
                out[k] = KEYS.has(k) ? '[REDACTED]' : walk(val);
            }
            return out;
        }
        return v;
    };
    return walk(payload);
}

// Plugin constants
const NAME = 'GADS';
const log = logger.getLogger(NAME); // Appium-support logger, namespaced to "GADS"

/**
 * GadsAppium
 *
 * Custom Appium plugin that:
 *  - Registers the Appium server instance with an external "GADS" hub
 *  - Mirrors all server logs to that hub, tagged by session
 */
class GadsAppium extends BasePlugin {
    static api = null;
    // Static property to hold the current session ID across all instances
    static currentSessionId = "";
    // Static property to hold the current driver
    static activeDriver = null;

    static actionLogSequence = 0; // Unique step counter per session so we can properly order logs
    static actionLogTenant = ''; // The tenant of the user creating the session for report filtering and access
    static actionLogBuildId = ''; // The ID of the current build run - comes from session capabilities
    static actionLogTestName = ''; // The name of the current test - comes from session capabilities
    static actionLogPlatformName = ''; // Name of the platform where tests are executed - Android, iOS, Tizen, etc
    static actionLogDeviceName = ''; // The name of the device on which the current session runs - comes from GADS in session capabilities
    static actionLogAppPackage = ''; // Android app package name for more info in reports - comes from session capabilities
    static actionLogBundleId = ''; // iOS app bundle identifier for more info in reports - comes from session capabilities

    // New endpoints on the Appium server from the plugin itself
    static newMethodMap = {
        '/gads/source': {
            GET: { command: 'getSourceNoSession', neverProxy: true }, // never proxy → Appium handles it
        },
    };

    // Clear the action log data on session end or whatever
    async clearActionLogData() {
        GadsAppium.actionLogSequence = 0;
        GadsAppium.actionLogTenant = '';
        GadsAppium.actionLogBuildId = '';
        GadsAppium.actionLogTestName = '';
        GadsAppium.actionLogPlatformName = '';
        GadsAppium.actionLogAppPackage = '';
        GadsAppium.actionLogBundleId = '';
        GadsAppium.actionLogDeviceName = '';
    }

    /**
     * updateServer
     *
     * Called once when Appium server starts up.
     * Registers the server with the hub, and configures log forwarding.
     *
     * @param {object} _app         placeholder for the Express app
     * @param {object} _httpServer  placeholder for the HTTP server
     * @param {object} cliArgs      Parsed CLI arguments
   */
    static async updateServer(_app, _httpServer, cliArgs) {
        // Load config from --plugin-gads-config or legacy --plugin.gads.config
        const cfg = loadConfig(
            cliArgs.pluginGadsConfig ?? cliArgs.plugin?.gads?.config
        );
        if (!cfg.providerUrl) throw new Error("GADS - 'providerUrl' missing in config");

        // Set baseURL dynamically
        GadsAppium.api = axios.create({
            baseURL: `${cfg.providerUrl}/device/${cfg.udid}/appium-plugin`
        });

        // Save config globally
        GadsAppium.cfg = cfg;

        // Attempt to register this Appium instance with the GADS hub
        try {
            await GadsAppium.api.post(`/register`, cfg)
                .catch((e) => {
                    throw new Error(`GADS - Registration session failed, provider down - ${e.message}`)
                })
            log.info(`Registering device at -> ${cfg.providerUrl}/register`);
        } catch (e) {
            log.warn(`Device registration failed: ${e.message}`);
        }

        // Hook into npm‐log (the global logger Appium uses) to forward logs
        const npmlog = /** @type {import('npmlog')} */ (global._global_npmlog);
        npmlog.disableColor();
        // For each log event, POST to /log endpoint (fire-and-forget)
        let logSeq = 0;
        npmlog.on('log', ({ level, message, prefix }) => {
            const seq = logSeq++;

            GadsAppium.api.post(`/log`, {
                level: level,
                message: message,
                session_id: GadsAppium.currentSessionId,
                prefix: prefix,
                timestamp: Date.now(),
                sequenceNumber: seq
            }).catch(() => { });
        });
        log.info(`Mirroring logs to -> ${cfg.providerUrl}/logs`)

        setInterval(() => {
            GadsAppium.api.post(`/ping`, {
                timestamp: Date.now(),
                session_id: GadsAppium.currentSessionId
            }).catch((e) => {
                throw new Error(`GADS - Heartbeat failed, provider down - ${e.message}`)
            })
        }, cfg.heartbeatIntervalMs)
    }

    /**
     * createSession
     *
     * Wraps the driver.createSession call to capture the generated sessionId and notify the provider
     * That sessionId is used to tag subsequent log messages.
     *
     * @param {Function} next            The next handler in the chain
     * @param {object} driver            The underlying driver instance
     * @param {object} jsonwpCaps        JSONWP-style capabilities
     * @param {object} reqCaps           W3C requested capabilities
     * @param {object} w3cCapabilities   W3C capabilities object
     * @returns {object}                 The result of driver.createSession
   */
    async createSession(next, driver, jsonwpCaps, reqCaps, w3cCapabilities) {
        // Make sure instance has access to the loaded config
        this.cfg = GadsAppium.cfg;

        // Call through to the driver’s createSession
        const createSessionResult = await driver.createSession?.(jsonwpCaps, reqCaps, w3cCapabilities);
        GadsAppium.activeDriver = driver;
        // Extract the sessionId
        const sessionId = createSessionResult?.value?.[0];
        if (sessionId) {
            GadsAppium.currentSessionId = sessionId;
            GadsAppium.api.post(`/session/add/${GadsAppium.currentSessionId}`)
                .catch((e) => {
                    throw new Error(`GADS - Add session failed, provider down - ${e.message}`)
                })
        }

        // We get the build id from capabilities
        const buildId = w3cCapabilities?.alwaysMatch?.['gads:buildId'];
        if (buildId) {
            // We store the build ID for the report logs
            GadsAppium.actionLogBuildId = buildId
            log.info(`GADS: Build ID set to: ${buildId}`);
        } else {
            // If we don't have a build ID we preventively clear the static action log data
            // So we don't save log
            this.clearActionLogData()
        }

        GadsAppium.actionLogSequence = 0;
        const tenant = w3cCapabilities?.alwaysMatch?.['gads:tenant']
        if (tenant) {
            GadsAppium.actionLogTenant = tenant
        }
        const testName = w3cCapabilities?.alwaysMatch?.['gads:testName']
        if (testName) {
            GadsAppium.actionLogTestName = testName
        }
        const platformName = w3cCapabilities?.alwaysMatch?.['platformName']
        if (platformName) {
            GadsAppium.actionLogPlatformName = platformName
        }
        const appPackage = w3cCapabilities?.alwaysMatch?.['appium:appPackage']
        if (appPackage) {
            GadsAppium.actionLogAppPackage = appPackage
        }
        const bundleId = w3cCapabilities?.alwaysMatch?.['appium:bundleId']
        if (bundleId) {
            GadsAppium.actionLogBundleId = bundleId
        }
        const deviceName = w3cCapabilities?.alwaysMatch?.['gads:deviceName']
        if (deviceName) {
            GadsAppium.actionLogDeviceName = deviceName
        }

        return createSessionResult;
    }

    /**
     * deleteSession
     *
     * Wraps driver.deleteSession to clear the stored sessionId when the session ends and notify the provider
     *
     * @param {Function} next       The next handler in the chain
     * @param {object} driver       The underlying driver instance
     * @param {string} sessionId    The sessionId to delete
   */
    async deleteSession(next, driver, sessionId) {
        this.clearActionLogData()
        // If we’re deleting the active session, clear it
        if (GadsAppium.currentSessionId === sessionId) {
            GadsAppium.currentSessionId = "";
        }
        GadsAppium.activeDriver = null;
        const deleteSessionResult = await driver.deleteSession?.(sessionId);
        GadsAppium.api.post(`/session/remove`)
            .catch((e) => {
                throw new Error(`GADS - Remove session failed, provider down - ${e.message}`)
            })
        return deleteSessionResult
    }

    /**
     * Wraps the handle function for driver commands so we can get data for session logging
     * @param {*} next 
     * @param {*} driver 
     * @param {*} commandName 
     * @param  {...any} args 
     * @returns 
     */
    async handle(next, driver, commandName, ...args) {
        if (['createSession', 'deleteSession', 'getSessions'].includes(commandName)) {
            return await next();
        }

        const t0 = Date.now();
        let result, error;

        try {
            result = await next();
        } catch (e) {
            error = e;
        }

        try {
            const cfg = GadsAppium.cfg || this.cfg;
            if (cfg?.providerUrl && cfg?.udid) {
                const info = classify(commandName, args);
                if (info && GadsAppium.actionLogBuildId) {
                    GadsAppium.actionLogSequence += 1;
                    let findElementUsing = null;
                    let findElementSelector = null;
                    if (commandName === 'findElement' || commandName === 'findElements') {
                        findElementUsing = args?.[0]
                        findElementSelector = args?.[1]
                    }
                    const body = {
                        timestamp: Date.now(),
                        session_id: GadsAppium.currentSessionId || null,
                        udid: cfg.udid,
                        action: info.action,
                        command: commandName,
                        source: info.source,
                        duration_ms: Date.now() - t0,
                        success: !error,
                        error: error ? String(error.message || error) : undefined,
                        args: redact(args),
                        sequence_number: GadsAppium.actionLogSequence,
                        tenant: GadsAppium.actionLogTenant,
                        build_id: GadsAppium.actionLogBuildId,
                        test_name: GadsAppium.actionLogTestName,
                        locator_using: findElementUsing,
                        locator_value: findElementSelector,
                        device_name: GadsAppium.actionLogDeviceName,
                        app_package: GadsAppium.actionLogAppPackage,
                        bundle_identifier: GadsAppium.actionLogBundleId,
                        platform_name: GadsAppium.actionLogPlatformName,
                    };
                    await GadsAppium.api.post(`/log-session`, body).catch(() => { });
                }
            }
        } catch (e) {
            log.error(`Something failed - ${e}`);
        }

        if (error) throw error;
        return result;
    }

    /**
     * onUnexpectedShutdown
     *
     * Wraps onUnexpectedShutdown which detects that the session's driver crashed (e.g., emulator dies, App quits abruptly) etc and notifies the provider
     *
     * @param {object} driver   Тhe underlying driver instance
     * @param {string} cause    The cause of the shutdown
   */
    async onUnexpectedShutdown(driver, cause) {
        log.warn(`GADS: Session ${GadsAppium.currentSessionId} crashed unexpectedly`)
        GadsAppium.currentSessionId = ""
        GadsAppium.activeDriver = null
        this.clearActionLogData()

        GadsAppium.api.post(`/session/remove`)
            .catch((e) => {
                throw new Error(`GADS - Remove session failed, provider down - ${e.message}`)
            })
    }

    /**x
     * metadata
     *
     * Describes the plugin for Appium’s `--use-plugins` flag.
   */
    static get metadata() {
        return { pluginName: NAME };
    }
}

export { GadsAppium };