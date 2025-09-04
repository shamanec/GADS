import { BasePlugin } from '@appium/base-plugin';
import { logger } from '@appium/support';
import { loadConfig } from './src/config/loader.js';
import { createApiClient, GadsApiClient } from './src/api/client.js';
import { classify } from './src/utils/classifier.js';
import { handleArgs } from './src/utils/commandArgsHandler.js';

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
    static apiClient = null;
    // Static property to hold the current Appium session ID
    static currentSessionId = "";
    // Static property to hold the current driver
    static activeDriver = null;

    // Static properties for the action logs we send to GADS
    static actionLogSequence = 0; // Unique step counter per session so we can properly order logs
    static actionLogTenant = ''; // The tenant of the user creating the session for report filtering and access
    static actionLogBuildId = ''; // The ID of the current build run - comes from session capabilities
    static actionLogTestName = ''; // The name of the current test - comes from session capabilities
    static actionLogPlatformName = ''; // Name of the platform where tests are executed - Android, iOS, Tizen, etc
    static actionLogDeviceName = ''; // The name of the device on which the current session runs - comes from GADS in session capabilities

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
        )

        // Create API client
        const axiosInstance = createApiClient(cfg)
        GadsAppium.apiClient = new GadsApiClient(axiosInstance)

        // Save config globally
        GadsAppium.cfg = cfg

        // Attempt to register this Appium instance with the GADS hub
        try {
            await GadsAppium.apiClient.register(cfg);
            log.info(`Registering device at -> ${cfg.providerUrl}/register`)
        } catch (e) {
            log.warn(`Device registration failed: ${e.message}`)
        }

        // Hook into npm‐log (the global logger Appium uses) to forward logs
        const npmlog = /** @type {import('npmlog')} */ (global._global_npmlog)
        npmlog.disableColor()

        // On server start setup a sequence number for proper ordering of logs
        // Because its possible that Appium sends multiple logs with the same timestamp
        let logSeq = 0

        // For each log event, POST to GADS provider /log endpoint (fire-and-forget)
        npmlog.on('log', ({ level, message, prefix }) => {
            // Increment the sequence number to ensure ordering of the logs when presented by GADS from the db
            const seq = logSeq++

            GadsAppium.apiClient.sendLog({
                level: level,
                message: message,
                session_id: GadsAppium.currentSessionId,
                prefix: prefix,
                timestamp: Date.now(),
                sequenceNumber: seq
            })
        })
        log.info(`Mirroring logs to -> ${cfg.providerUrl}/logs`)

        setInterval(() => {
            GadsAppium.apiClient.sendPing({
                timestamp: Date.now(),
                session_id: GadsAppium.currentSessionId
            });
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
        this.cfg = GadsAppium.cfg

        // Call through to the driver’s createSession
        const createSessionResult = await driver.createSession?.(jsonwpCaps, reqCaps, w3cCapabilities)

        // Store the active driver in case we need it for something
        GadsAppium.activeDriver = driver

        // Extract the sessionId
        const sessionId = createSessionResult?.value?.[0]
        if (sessionId) {
            GadsAppium.currentSessionId = sessionId;
            await GadsAppium.apiClient.addSession(GadsAppium.currentSessionId)
        }

        // Get the test run build ID from capabilities
        const buildId = w3cCapabilities?.alwaysMatch?.['gads:buildId']
        if (buildId) {
            // Store the build ID for the report logs
            GadsAppium.actionLogBuildId = buildId
            log.info(`GADS: Build ID set to: ${buildId}`)
        } else {
            // If we don't have a build ID we preventively clear the static action log data
            // So we don't save logs
            this.clearActionLogData()
        }

        // On a new session reset the action log sequence number
        GadsAppium.actionLogSequence = 0
        // Get the additional capabilities for extending the log information
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
        const deviceName = w3cCapabilities?.alwaysMatch?.['gads:deviceName']
        if (deviceName) {
            GadsAppium.actionLogDeviceName = deviceName
        }

        return createSessionResult
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
        // Clear all action log properties
        this.clearActionLogData()

        // If we’re deleting the active session, clear it from the properties
        if (GadsAppium.currentSessionId === sessionId) {
            GadsAppium.currentSessionId = ''
        }
        // Reset the active driver object
        GadsAppium.activeDriver = null;

        // Call through to the driver's deleteSession
        const deleteSessionResult = await driver.deleteSession?.(sessionId)

        // Notify GADS the session was deleted
        await GadsAppium.apiClient.removeSession()

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
        // Do not handle the commands below at all, directly call through to next()
        if (['createSession', 'deleteSession', 'getSessions'].includes(commandName)) {
            return await next()
        }

        // Handle execute command for test results before passing to driver
        if (commandName === 'execute') {
            const script = args?.[0]
            // First we check if the script is for storing test result for GADS
            // If not we just proceed with the usual handling
            if (typeof script === 'string' && script.includes('gads:testResult')) {
                try {
                    const cfg = GadsAppium.cfg || this.cfg
                    if (cfg?.providerUrl && cfg?.udid && GadsAppium.actionLogBuildId) {
                        // For executeScript, arguments are in an array as the second parameter
                        // So we parse the test results from there
                        const scriptArgs = args?.[1]
                        const testResult = Array.isArray(scriptArgs) ? scriptArgs[0] : scriptArgs
                        // We check if the test result is an object - probably can't be other but just in case
                        if (testResult && typeof testResult === 'object') {
                            // We build the test result body for storing the information in Mongo
                            const testResultBody = {
                                timestamp: Date.now(),
                                session_id: GadsAppium.currentSessionId || null,
                                udid: cfg.udid,
                                tenant: GadsAppium.actionLogTenant,
                                build_id: GadsAppium.actionLogBuildId,
                                test_name: GadsAppium.actionLogTestName || testResult.testName,
                                device_name: GadsAppium.actionLogDeviceName,
                                platform_name: GadsAppium.actionLogPlatformName,
                                status: testResult.status || 'unknown',
                                message: testResult.message || testResult.error || ''
                            }

                            // Send the test result to the provider for storing in Mongo
                            await GadsAppium.apiClient.sendTestResult(testResultBody)
                        }
                    }
                } catch (e) {
                    log.error(`Failed to process test result: ${e}`);
                }

                // Return success for gads:testResult scripts without calling next()
                return { value: 'Test result processed' };
            }
        }

        // Get the command start timestamp
        const commandStartTS = Date.now()
        let result, error;

        // Execute the command, saving result and/or error
        try {
            result = await next()
        } catch (e) {
            error = e
        }

        try {
            const cfg = GadsAppium.cfg || this.cfg
            if (cfg?.providerUrl && cfg?.udid) {
                // Classify the command turning the Appium command name into a human-readable report-friendly command name
                const info = classify(commandName, args)

                // We log actions only if command was classified and `gads:buildId` capability is provided
                if (info && GadsAppium.actionLogBuildId) {
                    // Increment the sequence number on each command we log
                    GadsAppium.actionLogSequence += 1

                    // Parse element locator data for findElement/findElements commands
                    let findElementUsing = null;
                    let findElementSelector = null;
                    if (commandName === 'findElement' || commandName === 'findElements') {
                        findElementUsing = args?.[0]
                        findElementSelector = args?.[1]
                    }

                    // Parse specific commands arguments into human readable output
                    const commandAdditionalInfo = handleArgs(commandName, args)

                    // Build the body of the log we send to GADS
                    const body = {
                        timestamp: commandStartTS, // Start time of the command
                        session_id: GadsAppium.currentSessionId || null, // Appium current session id
                        udid: cfg.udid, // Target device UDID
                        action: info.action, // Human-readable report-friendly command name
                        command: commandName, // Actual Appium command name
                        duration_ms: Date.now() - commandStartTS, // Time taken to execute the command
                        success: !error, // Is the command successful or not
                        error: error ? String(error.message || error) : null, // Error string if any
                        sequence_number: GadsAppium.actionLogSequence, // Log sequence number for proper logs ordering
                        tenant: GadsAppium.actionLogTenant, // Test execution target tenant name from GADS - `gads:tenant`
                        build_id: GadsAppium.actionLogBuildId, // Test execution build identifier - `gads:buildId`
                        test_name: GadsAppium.actionLogTestName, // Name of target test if any - `gads:testName`
                        locator_using: findElementUsing, // Type of target element locator for findElement/findElements
                        locator_value: findElementSelector, // Value of the target element locator for findElement/findElements
                        device_name: GadsAppium.actionLogDeviceName, // Target device name from GADS
                        platform_name: GadsAppium.actionLogPlatformName, // Target test platform name - iOS/Android/Tizen/WebOS
                        additional_info: commandAdditionalInfo, // Human-readable report-friendly additional info for specific commands
                    }

                    // Send the log to GADS
                    await GadsAppium.apiClient.sendSessionLog(body)
                }

            }
        } catch (e) {
            log.error(`Something failed - ${e}`)
        }

        // Keep the usual Appium handling - throw error or return result
        if (error) throw error
        return result
    }

    /**
     * onUnexpectedShutdown
     *
     * Wraps onUnexpectedShutdown which detects that the session's driver crashed (e.g., emulator dies, app quits abruptly) etc and notifies the provider
     *
     * @param {object} driver   Тhe underlying driver instance
     * @param {string} cause    The cause of the shutdown
   */
    async onUnexpectedShutdown(driver, cause) {
        log.warn(`GADS: Session ${GadsAppium.currentSessionId} crashed unexpectedly`)

        // Clear the session id, active driver and the static action log properties
        GadsAppium.currentSessionId = ""
        GadsAppium.activeDriver = null
        this.clearActionLogData()

        // Notify GADS the driver crashed by clearing the session on GADS side
        await GadsAppium.apiClient.removeSession()
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