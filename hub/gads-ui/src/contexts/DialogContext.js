import { Button, Dialog, DialogContent, DialogContentText, DialogTitle } from '@mui/material'
import { createContext, useContext, useState } from 'react'
import DialogActions from '@mui/material/DialogActions'

const DialogContext = createContext()

// The whole app is wrapped in this DialogProvider so we can reuse it to show different dialogs across the app.
function DialogProvider({ children }) {
    // `dialogs` is an object that holds the state for each dialog by a unique id (e.g., 'sessionAlert', 'errorDialog').
    // Each dialog has its own properties like title, content, actions, and isOpen status.
    const [dialogs, setDialogs] = useState({})

    // `showDialog` function opens a dialog by setting its properties and isOpen status.
    // `id` is a unique identifier for the dialog, and `dialogProps` includes dialog-specific options.
    const showDialog = (id, dialogProps) => {
        // Wrap the dialog actions to allow setting custom autoClose property on different dialog actions
        // Probably won't be used but just in case
        const wrappedActions = dialogProps.actions.map(action => ({
            ...action,
            onClick: () => {
                action.onClick() // Run the original onClick function
                if (action.autoClose !== false) { // Check autoClose, defaulting to true if undefined
                    hideDialog(id) // Close dialog if autoClose is true or not set
                }
            }
        }))

        setDialogs(prev => ({ ...prev, [id]: { ...dialogProps, actions: wrappedActions, isOpen: true } }))
    }

    // `hideDialog` function closes a dialog by setting its isOpen status to false for the given id.
    const hideDialog = (id) => {
        setDialogs(prev => ({ ...prev, [id]: { ...prev[id], isOpen: false } }))
    }

    return (
        // Provide the `showDialog` and `hideDialog` functions to the context, making them accessible to child components.
        <DialogContext.Provider value={{ showDialog, hideDialog }}>
            {children}

            {/* Render each dialog that's currently open by mapping through the `dialogs` state object */}
            {Object.keys(dialogs).map(id => (
                dialogs[id].isOpen && (
                    <Dialog
                        key={id} // Unique key for each dialog instance
                        open={dialogs[id].isOpen} // Control dialog visibility
                        onClose={dialogs[id].isCloseable ? () => hideDialog(id) : undefined} // Conditionally closeable
                        disableEscapeKeyDown={!dialogs[id].isCloseable} // Disables Escape key close if not closeable
                    >
                        <DialogTitle>{dialogs[id].title}</DialogTitle> {/* Display dialog title */}
                        <DialogContent>
                            <DialogContentText>{dialogs[id].content}</DialogContentText> {/* Display dialog content */}
                        </DialogContent>
                        <DialogActions>
                            {/* Map through the actions array to create buttons for each action */}
                            {dialogs[id].actions.map((action, index) => (
                                <Button key={index} onClick={action.onClick}>{action.label}</Button>
                            ))}
                        </DialogActions>
                    </Dialog>
                )
            ))}
        </DialogContext.Provider>
    )
}

function useDialog() {
    // Access the DialogContext. If it's not available, throw an error to ensure it's used correctly within the provider.
    const context = useContext(DialogContext)
    if (context === undefined) {
        throw new Error('useDialog must be used within a DialogProvider')
    }
    return context // Return the context value (i.e., `showDialog` and `hideDialog` functions)
}

export { DialogProvider, useDialog }
