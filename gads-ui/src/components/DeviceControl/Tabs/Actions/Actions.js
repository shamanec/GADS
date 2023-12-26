import { useContext, useState } from "react";
import Apps from "./Apps/Apps";
import { Alert, Box, CircularProgress, Stack, TextField } from "@mui/material";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";

export default function Actions({ deviceData }) {
    return (
        <Box marginTop='10px'>
            <Stack>
                <Apps deviceData={deviceData}></Apps>
                <TypeText deviceData={deviceData}></TypeText>
            </Stack>
        </Box>
    )
}

function TypeText({ deviceData }) {
    const [isTyping, setIsTyping] = useState(false)
    const [authToken, , logout] = useContext(Auth)
    const [showError, setShowError] = useState(false)

    function handleEnter(event) {
        // If currently typing text through the API with Appium do not allow typing in input box
        if (isTyping) {
            event.target.value = ""
        } else if (event.keyCode === 13) {
            if (event.target.value != "") {
                handleType(event.target.value)
                event.target.value = ""
            }
        }
    }

    function handleShowError() {
        setShowError(false)
        setShowError(true)
        setTimeout(() => {
            setShowError(false)
        }, 3000)
    }

    function handleType(text) {
        setIsTyping(true)
        let json = `{"text": "${text}"}`

        let url = `/device/${deviceData.Device.udid}/typeText`
        axios.post(url, json, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
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
                }, 1000)
            })
    }

    return (
        <Box style={{ backgroundColor: 'white', width: '600px', marginTop: '5px', height: '155px' }}>
            <div style={{ marginLeft: '10px', marginTop: '5px' }}>Make sure you've selected input element in the app</div>
            <Box style={{ display: 'flex', alignItems: 'center', marginLeft: '10px', marginBottom: '10px' }}>
                <TextField
                    id="outlined-basic"
                    label="Type something and press Enter"
                    variant="outlined"
                    onKeyUp={(event) => handleEnter(event)}
                    style={{ backgroundColor: 'white', marginTop: '15px', width: '80%' }}
                />
                {isTyping &&
                    <CircularProgress id='progress-indicator' size={45} />
                }
            </Box>
            {showError &&
                <Alert severity="error">Error typing or no active input element selected</Alert>
            }
        </Box>
    )
}