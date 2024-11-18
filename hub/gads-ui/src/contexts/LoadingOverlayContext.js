import React, { createContext, useState, useContext } from 'react'
import { CircularProgress, Backdrop } from '@mui/material'

const LoadingOverlayContext = createContext()

export function LoadingOverlayProvider({ children }) {
    const [isLoading, setIsLoading] = useState(false)

    const showLoadingOverlay = () => setIsLoading(true)
    const hideLoadingOverlay = () => {
        setTimeout(() => {
            setIsLoading(false)
        }, 2000)
    }

    return (
        <LoadingOverlayContext.Provider value={{ showLoadingOverlay, hideLoadingOverlay }}>
            {children}
            {/* Transparent overlay with spinner */}
            <Backdrop
                open={isLoading}
                style={{
                    color: '#fff',
                    zIndex: 1300, // Ensure it's on top of other components
                    backgroundColor: 'rgba(0, 0, 0, 0.5)', // Semi-transparent background
                }}
            >
                <CircularProgress color="inherit" /> {/* Loading spinner */}
            </Backdrop>
        </LoadingOverlayContext.Provider>
    );
}

export function useLoadingOverlay() {
    const context = useContext(LoadingOverlayContext);
    if (context === undefined) {
        throw new Error('useLoadingOverlay must be used within a LoadingOverlayProvider')
    }
    return context
}