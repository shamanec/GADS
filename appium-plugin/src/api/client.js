import axios from 'axios';

/**
 * Creates and configures an Axios API client for GADS communication
 * @param {object} config - Configuration object containing providerUrl and udid
 * @returns {object} Configured Axios instance
 * @throws {Error} When providerUrl is missing in config
 */
export function createApiClient(config) {
    if (!config.providerUrl) {
        throw new Error("GADS - 'providerUrl' missing in config");
    }

    return axios.create({
        baseURL: `${config.providerUrl}/device/${config.udid}/appium-plugin`
    })
}

/**
 * API operations wrapper for GADS communication
 */
export class GadsApiClient {
    constructor(axiosInstance) {
        this.api = axiosInstance
    }

    // Register the current device Appium server to the GADS provider instance
    async register(config) {
        try {
            await this.api.post('/register', config)
        } catch (e) {
            throw new Error(`GADS - Registration session failed, provider down - ${e.message}`)
        }
    }

    // Notify provider about the currently live session
    async addSession(sessionId) {
        try {
            await this.api.post(`/session/add/${sessionId}`);
        } catch (e) {
            throw new Error(`GADS - Add session failed, provider down - ${e.message}`);
        }
    }

    // Notify provider a session was ended
    async removeSession() {
        try {
            await this.api.post('/session/remove');
        } catch (e) {
            throw new Error(`GADS - Remove session failed, provider down - ${e.message}`)
        }
    }

    // Send Appium log to provider instance
    async sendLog(logData) {
        try {
            await this.api.post('/log', logData)
        } catch (e) {
            // Silent fail for logs to avoid disrupting main flow
        }
    }

    // Send Appium action log to provider instance
    async sendSessionLog(sessionLogData) {
        try {
            await this.api.post('/log-session', sessionLogData)
        } catch (e) {
            // Silent fail for session logs to avoid disrupting main flow
        }
    }

    // Send screenshot request to provider instance
    async sendScreenshotRequest(screenshotData) {
        try {
            await this.api.post('/screenshot', screenshotData)
        } catch (e) {
            // Silent fail for screenshots to avoid disrupting main flow
        }
    }


    // Ping to notify provider that the Appium server is live and running
    async sendPing(pingData) {
        try {
            await this.api.post('/ping', pingData);
        } catch (e) {
            throw new Error(`GADS - Heartbeat failed, provider down - ${e.message}`)
        }
    }
}