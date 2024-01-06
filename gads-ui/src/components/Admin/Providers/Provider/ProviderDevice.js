import { Box, Button, Stack } from "@mui/material";
import { useEffect, useState } from "react";

export default function ProviderDevice({ deviceInfo }) {
    let img_src = deviceInfo.Device.os === 'android' ? './images/android-logo.png' : './images/apple-logo.png'
    const [statusColor, setStatusColor] = useState('red')
    const [buttonDisabled, setButtonDisabled] = useState(false)

    useEffect(() => {
        if (deviceInfo.Device.connected && deviceInfo.ProviderState === 'live') {
            setStatusColor('green')
        } else if (deviceInfo.Device.connected && deviceInfo.ProviderState === 'preparing') {
            setStatusColor('orange')
        } else {
            setStatusColor('red')
        }
        if (deviceInfo.ProviderState !== 'init') {
            setButtonDisabled(false)
        } else {
            setButtonDisabled(true)
        }
    }, [])

    function handleResetClick() {
        let url = `/`
    }

    return (
        <Box style={{ width: '360px', margin: '5px', border: '1px solid black' }}>
            <Stack direction='row'>
                <div style={{ height: '60px', display: 'flex', justifyContent: 'left', alignItems: 'center' }}>
                    <OSImage img_src={img_src}></OSImage>
                    <div style={{ width: '30px', height: '30px', borderRadius: '50%', backgroundColor: `${statusColor}` }}></div>
                </div>
            </Stack>
            <div>UDID</div>
            <div>{deviceInfo.Device.udid}</div>
            <div>Last provider state: {deviceInfo.ProviderState}</div>
            <div>Name: {deviceInfo.Device.name}</div>
            <div>Width: {deviceInfo.Device.screen_width}</div>
            <div>Height: {deviceInfo.Device.screen_height}</div>
            <Button onClick={handleResetClick} disabled={buttonDisabled}>Reset</Button>
        </Box >
    )
}

function OSImage({ img_src }) {
    return (
        <img src={img_src} height='50px' alt='OS logo'></img>
    )
}