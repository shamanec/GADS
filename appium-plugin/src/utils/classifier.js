import { logger } from '@appium/support'

const log = logger.getLogger('GADS')

/**
 * This function classifies the type of action performed that we can use for session logging
 * @param {string} commandName The name of the execute command
 * @param {*} args The arguments for the command
 * @returns {object|null} Classification object with action property, or null if not classifiable
 */
export function classify(commandName, args) {
    log.info(`Classifying command ${commandName}`)

    switch (commandName) {
        // Advanced Actions (W3C Actions API)
        case 'performActions':
            return { action: 'Perform Actions' }

        // Custom script commands
        case 'executeScript': {
            const script = args?.[0];
            if (typeof script === 'string' && script.startsWith('mobile:')) {
                return { action: 'Execute Script' }
            }
            return null;
        }

        // Find element(s) commands
        case 'findElement':
            return { action: 'Find Element' }
        case 'findElements':
            return { action: 'Find Elements' }


        // Element Interaction Commands
        case 'click':
            return { action: 'Tap' }
        case 'setValue':
            return { action: 'Type' }
        case 'getText':
            return { action: 'Get Text' }
        case 'getAttribute':
            return { action: 'Get Attribute' }
        case 'sendKeys':
            return { action: 'Send Keys' }
        case 'clear':
            return { action: 'Clear Field' }
        case 'isDisplayed':
            return { action: 'Check Visibility' }
        case 'isEnabled':
            return { action: 'Check Enabled State' }
        case 'isSelected':
            return { action: 'Check Selection State' }
        case 'getRect':
            return { action: 'Get Element Bounds' }
        case 'getSize':
            return { action: 'Get Element Size' }
        case 'getLocation':
            return { action: 'Get Element Position' }

        // Navigation & App Control Commands
        case 'back':
            return { action: 'Navigate Back' }
        case 'forward':
            return { action: 'Navigate Forward' }
        case 'refresh':
            return { action: 'Refresh Page' }
        case 'getCurrentUrl':
            return { action: 'Get Current URL' }
        case 'getTitle':
            return { action: 'Get Page Title' }
        case 'quit':
            return { action: 'Quit Session' }

        // Device Actions
        case 'shake':
            return { action: 'Shake Device' }
        case 'lock':
            return { action: 'Lock Device' }
        case 'unlock':
            return { action: 'Unlock Device' }
        case 'pressKeyCode':
            return { action: 'Press Key' }
        case 'longPressKeyCode':
            return { action: 'Long Press Key' }
        case 'hideKeyboard':
            return { action: 'Hide Keyboard' }
        case 'isKeyboardShown':
            return { action: 'Check Keyboard State' }
        case 'getClipboard':
            return { action: 'Get Clipboard' }

        // App Management Commands
        case 'activateApp':
            return { action: 'Activate App' }
        case 'terminateApp':
            return { action: 'Terminate App' }
        case 'installApp':
            return { action: 'Install App' }
        case 'removeApp':
            return { action: 'Remove App' }
        case 'backgroundApp':
            return { action: 'Background App' }
        case 'queryAppState':
            return { action: 'Query App State' }
        case 'getCurrentPackage':
            return { action: 'Get Current Package' }
        case 'startActivity':
            return { action: 'Start Activity' }

        default:
            return { action: 'Other' }
    }
}