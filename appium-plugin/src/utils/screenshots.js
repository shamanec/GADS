import { logger } from '@appium/support'

const log = logger.getLogger('GADS')

/**
 * Takes a screenshot before command execution for specific commands
 * @param {object} GadsAppium - The main plugin class with static properties
 * @param {string} commandName - The Appium command name
 * @param {object} apiClient - The API client instance
 * @returns {boolean} True if screenshot was requested, false otherwise
 */
export function takeBeforeScreenshot(GadsAppium, commandName, apiClient) {
    const cfg = GadsAppium.cfg

    // Check if MinIO is enabled first - no point in other checks if screenshots are disabled
    if (!cfg?.minioEnabled) {
        return false
    }

    // Only take before screenshots for click and findElement commands
    if (commandName !== 'click' && commandName !== 'findElement') {
        return false
    }

    if (!cfg?.providerUrl || !cfg?.udid || !GadsAppium.actionLogBuildId) {
        return false
    }

    // Use the next sequence number (which will be assigned to the main log)
    const nextSequence = GadsAppium.actionLogSequence + 1
    const beforeScreenshotData = {
        session_id: GadsAppium.currentSessionId,
        build_id: GadsAppium.actionLogBuildId,
        sequence_number: nextSequence.toString(),
        is_after_command: false
    }

    // Fire and forget - don't block Appium execution
    apiClient.sendScreenshotRequest(beforeScreenshotData).catch(screenshotError => {
        log.warn(`Failed to send before screenshot request for ${commandName} command: ${screenshotError.message}`)
    })

    return true
}

/**
 * Takes a screenshot after command execution (for failed commands)
 * @param {object} GadsAppium - The main plugin class with static properties
 * @param {string} commandName - The Appium command name
 * @param {object} apiClient - The API client instance
 * @returns {boolean} True if screenshot was requested, false otherwise
 */
export function takeAfterScreenshot(GadsAppium, commandName, apiClient) {
    const cfg = GadsAppium.cfg

    // Check if MinIO is enabled first - no point in other checks if screenshots are disabled
    if (!cfg?.minioEnabled) {
        return false
    }

    if (!cfg?.providerUrl || !cfg?.udid || !GadsAppium.actionLogBuildId) {
        return false
    }

    const errorScreenshotData = {
        session_id: GadsAppium.currentSessionId,
        build_id: GadsAppium.actionLogBuildId,
        sequence_number: GadsAppium.actionLogSequence.toString(),
        is_after_command: true
    }

    // Fire and forget - don't block Appium execution
    apiClient.sendScreenshotRequest(errorScreenshotData).catch(screenshotError => {
        log.warn(`Failed to send error screenshot request for ${commandName} command: ${screenshotError.message}`)
    })

    return true
}