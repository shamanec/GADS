import React, { createContext, useState, useContext } from 'react'
import { Snackbar, Alert } from '@mui/material'

const SnackbarContext = createContext()

export function SnackbarProvider({ children }) {
    const [snackbar, setSnackbar] = useState({
        message: '',
        severity: 'info',
        isOpen: false,
        duration: 3000, // Snackbar auto-hide duration in milliseconds
    })

    // Show Snackbar function
    const showSnackbar = ({ message, severity = 'info', duration = 3000 }) => {
        // Close any currently open snackbar
        setSnackbar(prev => ({ ...prev, isOpen: false }))

        // After a short delay, show the new snackbar
        setTimeout(() => {
            setSnackbar({
                message,
                severity,
                isOpen: true,
                duration,
            })
        }, 100) // Delay to ensure previous snackbar is fully closed
    }

    // Hide Snackbar function
    const hideSnackbar = () => {
        setSnackbar(prev => ({ ...prev, isOpen: false }))
    }

    return (
        <SnackbarContext.Provider value={{ showSnackbar, hideSnackbar }}>
            {children}

            {/* Snackbar component */}
            <Snackbar
                open={snackbar.isOpen}
                autoHideDuration={snackbar.duration}
                onClose={(event, reason) => {
                    if (reason === 'timeout') {
                        hideSnackbar() // Only hide on timeout
                    }
                }}
                anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
            >
                <Alert onClose={hideSnackbar} severity={snackbar.severity} sx={{ width: '100%' }}>
                    {snackbar.message}
                </Alert>
            </Snackbar>
        </SnackbarContext.Provider>
    )
}

export function useSnackbar() {
    const context = useContext(SnackbarContext)
    if (context === undefined) {
        throw new Error('useSnackbar must be used within a SnackbarProvider')
    }
    return context
}