import { Box, Button, CircularProgress, Stack, TextField } from '@mui/material'
import { api } from '../../../../services/api'
import { useState } from 'react'
import { useSnackbar } from '../../../../contexts/SnackBarContext'


export default function Clipboard({ deviceData }) {
    const [isGettingCb, setIsGettingCb] = useState(false)
    const { showSnackbar } = useSnackbar()

    function handleGetClipboard() {
        setIsGettingCb(true)
        const url = `/device/${deviceData.udid}/getClipboard`
        api.get(url)
            .then((response) => {
                navigator.clipboard.writeText(response.data.message)
                setIsGettingCb(false)
                showSnackbar({
                    message: 'Device clipboard copied!',
                    severity: 'success',
                    duration: 2000,
                })
            })
            .catch(() => {
                setIsGettingCb(false)
                showSnackbar({
                    message: 'Failed to get device clipboard!',
                    severity: 'error',
                    duration: 2000,
                })
            })
    }

    return (
        <Box
            marginTop='10px'
            style={{
                backgroundColor: '#9ba984',
                width: '600px'
            }}
        >
            <Stack
                style={{
                    marginLeft: '10px',
                    marginBottom: '10px',
                    marginRight: '10px'
                }}
            >
                <h3>Get clipboard value</h3>
                {deviceData.os === 'ios' &&
                    <h5 style={{ marginTop: '1px' }}>On iOS devices WebDriverAgent has to be the active app to get the pasteboard value, it will be activated and then you'll be navigated to Springboard</h5>
                }
                <Button
                    onClick={handleGetClipboard}
                    id='clipboard-button'
                    variant='contained'
                    disabled={isGettingCb}
                    style={{
                        backgroundColor: isGettingCb ? 'rgba(51,71,110,0.47)' : '#2f3b26',
                        color: '#9ba984',
                        fontWeight: 'bold'
                    }}
                >
                    {isGettingCb ? (
                        <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                    ) : (
                        'Get clipboard'
                    )}
                </Button>
            </Stack>
        </Box>
    )
}