import { Button, Dialog, DialogContent, DialogContentText, DialogTitle } from "@mui/material";
import { createContext, useContext, useState } from "react";
import DialogActions from '@mui/material/DialogActions';
import { Auth } from "../../contexts/Auth";
import { useNavigate } from "react-router-dom";
import { api } from '../../services/api.js'

const DialogContext = createContext()

function SessionAlert({ dialog, setDialog }) {
    const [authToken, , , , logout] = useContext(Auth)
    const [isRefreshing, setIsRefreshing] = useState(false)
    const navigate = useNavigate()

    function hideDialog() {
        setDialog(false)
    }

    function refreshSession() {
        let healthURL = `/health`
        api.get(healthURL, {})
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                }
                navigate('/devices')
            })
        setDialog(false)
    }

    function backToDevices() {
        navigate('/devices')
    }

    return (
        <Dialog
            open={dialog}
            onClose={hideDialog}
            aria-labelledby="alert-dialog-title"
            aria-describedby="alert-dialog-description"
        >
            <DialogTitle id="alert-dialog-title">
                {"Session lost!"}
            </DialogTitle>
            <DialogContent>
                <DialogContentText id="alert-dialog-description">
                    You should navigate back to the devices list
                </DialogContentText>
            </DialogContent>
            <DialogActions>
                <Button variant='contained' onClick={backToDevices}>Back to devices</Button>
                {/* <Button variant='contained' onClick={refreshSession} autoFocus>
                    Refresh session
                </Button> */}
            </DialogActions>
        </Dialog>
    )
}

function DialogProvider({ children }) {
    const [dialog, setDialog] = useState()

    return (
        <DialogContext.Provider value={{ setDialog }} >
            {children}
            {dialog && <SessionAlert dialog={dialog} setDialog={setDialog} />}
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
