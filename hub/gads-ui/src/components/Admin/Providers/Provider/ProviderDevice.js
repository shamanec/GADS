import { Box, Button, Stack } from "@mui/material";
import { useContext, useEffect, useState } from "react";
import { Auth } from "../../../../contexts/Auth";
import { api } from '../../../../services/api.js'

export default function ProviderDevice({ deviceInfo }) {
    let img_src = deviceInfo.os === 'android' ? './images/android-logo.png' : './images/apple-logo.png'
    const [statusColor, setStatusColor] = useState('red')
    const [buttonDisabled, setButtonDisabled] = useState(false)
    const [authToken, , , , logout] = useContext(Auth)

    useEffect(() => {
        if (deviceInfo.connected && deviceInfo.provider_state === 'live') {
            setStatusColor('green')
        } else if (deviceInfo.connected && deviceInfo.provider_state === 'preparing') {
            setStatusColor('orange')
        } else {
            setStatusColor('red')
        }
        if (deviceInfo.provider_state !== 'init') {
            setButtonDisabled(false)
        } else {
            setButtonDisabled(true)
        }
    })

    function handleResetClick() {
        let url = `/device/${deviceInfo.udid}/reset`

        api.post(url)
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                    }
                }
            })
        }

    return (
        <Box
            style={{
                width: '360px',
                margin: '5px',
                border: '1px solid black'
            }}
        >
            <Stack direction='row'>
                <div
                    style={{
                        height: '60px',
                        display: 'flex',
                        justifyContent: 'left',
                        alignItems: 'center'
                    }}
                >
                    <OSImage img_src={img_src}></OSImage>
                    <div
                        style={{
                            width: '30px',
                            height: '30px',
                            borderRadius: '50%',
                            backgroundColor: `${statusColor}`
                        }}
                    ></div>
                </div>
            </Stack>
            <div>UDID</div>
            <div>{deviceInfo.udid}</div>
            <div>Last provider state: {deviceInfo.provider_state}</div>
            <div>Name: {deviceInfo.name}</div>
            <div>Width: {deviceInfo.screen_width}</div>
            <div>Height: {deviceInfo.screen_height}</div>
            <Button
                onClick={handleResetClick}
                disabled={buttonDisabled}
                variant='contained'
                style={{
                    margin: "2px" ,
                    backgroundColor: "#0c111e",
                    color: "#78866B",
                    fontWeight: "bold"
                }}
            >Reset</Button>
        </Box >
    )
}

function OSImage({ img_src }) {
    return (
        <img
            src={img_src}
            height='50px'
            alt='OS logo'
        ></img>
    )
}