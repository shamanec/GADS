import ShowFailedSessionAlert from "../SessionAlert";
import { Box } from "@mui/material";
import { Button } from "@mui/material";
import { Stack } from "@mui/material";
import { useState, useContext } from "react";
import { Auth } from "../../../contexts/Auth";

export default function Screenshot({ udid }) {
    const [authToken, , logout] = useContext(Auth)
    const [width, setWidth] = useState(0)
    const [height, setHeight] = useState(0)

    function takeScreenshot() {
        const url = `/device/${udid}/screenshot`;
        fetch(url, {
            method: 'POST',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                if (response.status === 404) {
                    ShowFailedSessionAlert()
                    return
                }
                return response.json()
            })
            .then(screenshotJson => {
                var imageBase64String = screenshotJson.value
                let image = document.getElementById('screenshot-image')
                image.src = "data:image/png;base64," + imageBase64String
                image = document.getElementById('screenshot-image')
                setWidth(image.width)
                setHeight(image.height)
            })
            .catch(error => {
                console.log('could not take screenshot - ' + error)
            })
    }

    return (
        <Box style={{ marginTop: "20px" }}>
            <Stack style={{ height: "800px", width: "650px" }}>
                <Button onClick={() => takeScreenshot(udid)} variant="contained" style={{ marginBottom: "10px" }}>Screenshot</Button>
                <div style={{ overflowY: 'auto', height: "auto", position: "relative" }}>
                    <img id="screenshot-image" style={{ maxWidth: "100%", width: "auto", height: "auto" }} />
                </div>

            </Stack>
        </Box>
    )
}

