import { useContext, useState } from 'react'
import Apps from './Apps/Apps'
import { Alert, Box, CircularProgress, Stack, TextField } from '@mui/material'
import { Auth } from '../../../../contexts/Auth'
import { api } from '../../../../services/api.js'
import Clipboard from './Clipboard'

export default function Actions({ deviceData }) {
    return (
        <Box marginTop='10px'>
            <Stack>
                <Apps deviceData={deviceData}></Apps>
                <TypeText deviceData={deviceData}></TypeText>
                <Clipboard deviceData={deviceData}></Clipboard>
            </Stack>
        </Box>
    )
}

function TypeText({ deviceData }) {
    const [isTyping, setIsTyping] = useState(false)
    const { logout } = useContext(Auth)
    const [showError, setShowError] = useState(false)
    const [alertTimeoutId, setAlertTimeoutId] = useState(null)

    function handleEnter(event) {
        // If currently typing text through the API with Appium do not allow typing in input box
        if (isTyping) {
            event.target.value = ''
        } else if (event.keyCode === 13) {
            if (event.target.value != '') {
                handleType(event.target.value)
                event.target.value = ''
            }
        }
    }

    // Show error for the typing
    function handleShowError() {
        // Hide the previous error
        setShowError(false)
        // Show the current error
        setShowError(true)
        // Clear the previous timeout set on the previous error
        clearTimeout(alertTimeoutId)
        // Set a new timeout on the error
        setAlertTimeoutId(setTimeout(() => {
            setShowError(false)
        }, 3000))
    }

    function handleType(text) {
        setIsTyping(true)
        setShowError(false)

        let json = `{'text': '${text}'}`

        let url = `/device/${deviceData.udid}/typeText`
        api.post(url, json)
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                    if (error.response.status === 404) {
                    }
                    handleShowError()
                }
            })
            .finally(() => {
                setTimeout(() => {
                    setIsTyping(false)
                }, 500)
            })
    }

    return (
        <Box
            style={{
                backgroundColor: '#9ba984',
                width: '600px',
                marginTop: '5px',
                height: '155px'
            }}
        >
            <div
                style={{
                    marginLeft: '10px',
                    marginTop: '5px'
                }}
            >Make sure you've selected input element in the app</div>
            <Box
                style={{
                    marginLeft: '10px'
                }}
            >
                <TextField
                    id='outlined-basic'
                    label='Type something and press Enter'
                    variant='outlined'
                    onKeyUp={(event) => handleEnter(event)}
                    style={{
                        backgroundColor: '#9ba984',
                        marginTop: '15px',
                        width: '80%'
                    }}
                />
                {isTyping &&
                    <CircularProgress
                        variant='indeterminate'
                        size={40}
                        style={{
                            animationDuration: '600ms',
                            marginTop: '20px',
                            marginLeft: '5px'
                        }}
                    />
                }
            </Box>
            {showError &&
                <Alert
                    severity='error'
                    sx={{
                        width: '80%',
                        marginTop: '5px',
                        marginLeft: '10px'
                    }}
                >Error typing or no active input element selected</Alert>
            }
        </Box>
    )
}