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

    // New endpoints on the Appium server from the plugin itself
    static newMethodMap = {
        '/gads/source': {
            GET: { command: 'getSourceNoSession', neverProxy: true }, // never proxy → Appium handles it
        },
    };

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

        // Extract the sessionId
        const sessionId = createSessionResult?.value?.[0]
        if (sessionId) {
            GadsAppium.currentSessionId = sessionId;
            await GadsAppium.apiClient.addSession(GadsAppium.currentSessionId)
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
     * Wraps the handle function for driver commands so we can get data for session logging
     * @param {*} next 
     * @param {*} driver 
     * @param {*} commandName 
     * @param  {...any} args 
     * @returns 
     */
    async handle(next, driver, commandName, ...args) {
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