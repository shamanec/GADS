/**
 * This is a function that provides additional info on the executed command by parsing certain command arguments into human-readable output
 * @param {*} commandName 
 * @param {*} args 
 * @returns human readable output of the command arguments or null
 */
export function handleArgs(commandName, args) {
    if (args && args.length > 0) {
        switch (commandName) {
            case 'setValue':
                return `Setting element value to '${args?.[0]}'`
            case 'performActions':
                const actionsData = args?.[0]
                if (actionsData && typeof actionsData === 'object') {
                    return JSON.stringify(actionsData)
                } else {
                    return 'No info'
                }
            case 'executeScript': {
                const script = args?.[0]
                if (typeof script === 'string' && script.startsWith('mobile:')) {
                    return `Script: ${script}`
                }
                return 'No info'
            }
            case 'activateApp':
                return `Activating app with id '${args?.[0]}'`
            case 'terminateApp':
                return `Killing app with id '${args?.[0]}'`
            case 'installApp':
                return `Installing app with path '${args?.[0]}'`
            case 'removeApp':
                return `Removing app with id '${args?.[0]}'`
            case 'backgroundApp':
                return `Putting current app to background for '${args?.[0]}' seconds`
            default:
                return null
        }
    } else {
        return null
    }
}