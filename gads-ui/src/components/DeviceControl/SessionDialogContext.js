import { Button, Dialog, DialogContent, DialogContentText, DialogTitle } from "@mui/material";
import { createContext, useCallback, useContext, useState } from "react";
import DialogActions from '@mui/material/DialogActions';

const DialogContext = createContext()

function SessionAlert({ dialog, unsetDialog }) {
    return (
        <Dialog
            open={dialog}
            onClose={unsetDialog}
            aria-labelledby="alert-dialog-title"
            aria-describedby="alert-dialog-description"
        >
            <DialogTitle id="alert-dialog-title">
                {"Use Google's location service?"}
            </DialogTitle>
            <DialogContent>
                <DialogContentText id="alert-dialog-description">
                    Let Google help apps determine location. This means sending anonymous
                    location data to Google, even when no apps are running.
                </DialogContentText>
            </DialogContent>
            <DialogActions>
                <Button onClick={unsetDialog}>Disagree</Button>
                <Button onClick={unsetDialog} autoFocus>
                    Agree
                </Button>
            </DialogActions>
        </Dialog>
    )
}

function DialogProvider({ children }) {
    const [dialog, setDialog] = useState()

    const unsetDialog = useCallback(() => {
        setDialog()
    }, [setDialog])

    return (
        <DialogContext.Provider value={{ unsetDialog, setDialog }} >
            {children}
            {dialog && <SessionAlert dialog={dialog} unsetDialog={unsetDialog} />}
        </DialogContext.Provider>
    )
}

function useDialog() {
    const context = useContext(DialogContext)
    if (context === undefined) {
        throw new Error('useModal must be used within a UserProvider')
    }

    return context
}

export { DialogProvider, useDialog }
