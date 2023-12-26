import { useContext, useState } from "react";
import Apps from "./Apps/Apps";
import { Box, CircularProgress, Stack, TextField } from "@mui/material";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";
import { useDialog } from "../../SessionDialogContext";

export default function Actions({ deviceData }) {
    return (
        <Box marginTop='10px'>
            <Stack>
                <Apps deviceData={deviceData}>

                </Apps>
                <TypeText deviceData={deviceData}></TypeText>
            </Stack>
        </Box>
    )
}

function TypeText({ deviceData }) {
    const { setDialog } = useDialog()
    const [isTyping, setIsTyping] = useState(false)
    const [authToken, , logout] = useContext(Auth)

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
                        // setDialog(true)
                    }
                }
            })
            .finally(() => {
                setTimeout(() => {
                    setIsTyping(false)
                }, 500)
            })
    }

    return (
        <Box style={{ backgroundColor: 'white', width: '600px', borderRadius: '10px', marginTop: '5px' }}>
            <div style={{ marginLeft: '10px', marginTop: '5px' }}>Make sure you've selected input element in the app</div>
            <Box style={{ display: 'flex', alignItems: 'center', marginLeft: '10px', marginBottom: '10px' }}>
                <TextField
                    id="outlined-basic"
                    label="Type something and press Enter"
                    variant="outlined"
                    onKeyUp={(event) => handleEnter(event)}
                    style={{ backgroundColor: 'white', marginTop: '15px', width: '80%', borderBottomLeftRadius: '5px', borderBottomRightRadius: '5px' }}
                />
                {isTyping &&
                    <CircularProgress id='progress-indicator' size={45} />
                }
            </Box>
        </Box>
    )
}