import { BasePlugin } from '@appium/base-plugin';
import { logger } from '@appium/support';
import { loadConfig } from './src/config/loader.js';
import { createApiClient, GadsApiClient } from './src/api/client.js';

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
    // Timestamp of the last throttled command-activity report to the provider
    static lastCommandReportTs = 0;

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
        // Load config from --plugin-gads-config, which Appium's CLI parser exposes
        // as the nested plugin.gads.config; the flat camelCase key is kept as a
        // fallback for safety
        const cfg = loadConfig(
            cliArgs.plugin?.gads?.config ?? cliArgs.pluginGadsConfig
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
     * Wraps session creation to capture the generated sessionId and notify the provider.
     * That sessionId is used to tag subsequent log messages. The resolved session
     * capabilities are sent along so GADS knows what the driver actually started with.
     *
     * Calls through via next() (canonical plugin chaining) - the capability arguments
     * differ between Appium 2 and 3, but the result shape {value: [sessionId, caps,
     * protocol]} is identical across both majors.
     *
     * @param {Function} next   The next handler in the chain
     * @param {object} driver   The underlying driver instance
     * @returns {object}        The session creation result
   */
    async createSession(next, driver) {
        // Make sure instance has access to the loaded config
        this.cfg = GadsAppium.cfg

        const createSessionResult = await next()

        // Extract the sessionId and the resolved capabilities
        const sessionId = createSessionResult?.value?.[0]
        if (sessionId) {
            GadsAppium.currentSessionId = sessionId;
            await GadsAppium.apiClient.addSession(sessionId, createSessionResult?.value?.[1])
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
        // If we’re deleting the active session, clear it from the properties
        if (GadsAppium.currentSessionId === sessionId) {
            GadsAppium.currentSessionId = ''
        }

        // Call through to the driver's deleteSession
        const deleteSessionResult = await driver.deleteSession?.(sessionId)

        // Notify GADS the session was deleted
        await GadsAppium.apiClient.removeSession()

        return deleteSessionResult
    }

    /**
     * Wraps every driver command (including proxied ones, since a generic handle()
     * disables Appium's direct proxying) and reports activity to the provider,
     * throttled to at most one report per 2 seconds. The report is fire-and-forget -
     * awaiting it would add provider round-trip latency to every command.
     * @param {*} next
     * @param {*} driver
     * @param {*} commandName
     * @param  {...any} args
     * @returns
     */
    async handle(next, driver, commandName, ...args) {
        const now = Date.now()
        if (now - GadsAppium.lastCommandReportTs > 2000) {
            GadsAppium.lastCommandReportTs = now
            GadsAppium.apiClient.sendCommand({
                command: commandName,
                session_id: GadsAppium.currentSessionId,
                timestamp: now,
            })
        }
        return await next()
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

        // Clear the session id and the static action log properties
        GadsAppium.currentSessionId = ""

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