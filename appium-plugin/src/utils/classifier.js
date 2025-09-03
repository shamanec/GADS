import { logger } from '@appium/support';

const log = logger.getLogger('GADS');

/**
 * This function classifies the type of action performed that we can use for session logging
 * @param {string} commandName The name of the execute command
 * @param {*} args The arguments for the command
 * @returns {object|null} Classification object with action property, or null if not classifiable
 */
export function classify(commandName, args) {
    log.info(`Classifying command ${commandName}`);
    
    switch (commandName) {
        case 'click': 
            return { action: 'Tap' };
        case 'setValue': 
            return { action: 'Type' };
        case 'performActions': 
            return { action: 'Gesture' };
        case 'executeScript': {
            const script = args?.[0];
            if (typeof script === 'string' && script.startsWith('mobile:')) {
                return { action: 'Mobile-command' };
            }
            return null;
        }
        case 'findElement': 
            return { action: 'Find Element' };
        case 'findElements': 
            return { action: 'Find Elements' };
        case 'getClipboard': 
            return { action: 'Get Clipboard' };
        
        // Element Interaction Commands
        case 'getText': 
            return { action: 'Get Text' };
        case 'getAttribute': 
            return { action: 'Get Attribute' };
        case 'sendKeys': 
            return { action: 'Send Keys' };
        case 'clear': 
            return { action: 'Clear Field' };
        case 'isDisplayed': 
            return { action: 'Check Visibility' };
        case 'isEnabled': 
            return { action: 'Check Enabled State' };
        case 'isSelected': 
            return { action: 'Check Selection State' };
        case 'getRect': 
            return { action: 'Get Element Bounds' };
        case 'getSize': 
            return { action: 'Get Element Size' };
        case 'getLocation': 
            return { action: 'Get Element Position' };
        
        // Navigation & App Control Commands
        case 'back': 
            return { action: 'Navigate Back' };
        case 'forward': 
            return { action: 'Navigate Forward' };
        case 'refresh': 
            return { action: 'Refresh Page' };
        case 'getCurrentUrl': 
            return { action: 'Get Current URL' };
        case 'getTitle': 
            return { action: 'Get Page Title' };
        case 'quit': 
            return { action: 'Quit Session' };
        
        // Device Actions
        case 'shake': 
            return { action: 'Shake Device' };
        case 'lock': 
            return { action: 'Lock Device' };
        case 'unlock': 
            return { action: 'Unlock Device' };
        case 'pressKeyCode': 
            return { action: 'Press Key' };
        case 'longPressKeyCode': 
            return { action: 'Long Press Key' };
        case 'hideKeyboard': 
            return { action: 'Hide Keyboard' };
        case 'isKeyboardShown': 
            return { action: 'Check Keyboard State' };
        
        // Advanced Actions
        case 'swipe': 
            return { action: 'Swipe Gesture' };
        case 'scroll': 
            return { action: 'Scroll Action' };
        case 'pinch': 
            return { action: 'Pinch Gesture' };
        case 'zoom': 
            return { action: 'Zoom Gesture' };
        case 'touchAction': 
            return { action: 'Touch Action' };
        case 'multiAction': 
            return { action: 'Multi Touch' };
        
        // App Management Commands
        case 'activateApp': 
            return { action: 'Activate App' };
        case 'terminateApp': 
            return { action: 'Terminate App' };
        case 'installApp': 
            return { action: 'Install App' };
        case 'removeApp': 
            return { action: 'Remove App' };
        case 'backgroundApp': 
            return { action: 'Background App' };
        case 'queryAppState': 
            return { action: 'Query App State' };
        case 'getCurrentPackage': 
            return { action: 'Get Current Package' };
        case 'startActivity': 
            return { action: 'Start Activity' };
        
        default: 
            return null;
    }
}