import { Box, Button, Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import { useContext, useEffect, useState } from "react";
import { Auth } from "../../../../contexts/Auth";
import ProviderInfo from "./ProviderInfo";
import ProviderDevice from "./ProviderDevice"

export default function Provider({ info }) {
    const [authToken, , , , logout] = useContext(Auth)

    return (
        <Stack id='koleo' style={{ marginTop: '10px', marginBottom: '10px', borderRadius: '10px', padding: '10px' }}>
            <Stack direction='row' spacing={2}>
                <ProviderConfig
                    isNew={false}
                    data={info}
                >
                </ProviderConfig>
                <LiveProviderBox nickname={info.nickname} os={info.os}></LiveProviderBox>
            </Stack>
        </Stack>
    )
}

function InfoBox({ os, isOnline }) {
    return (
        <ProviderInfo os={os} isOnline={isOnline}></ProviderInfo>
    )
}


function LiveProviderBox({ nickname, os }) {
    let infoSocket = null;
    let [devicesData, setDevicesData] = useState(null)
    const [isLoading, setIsLoading] = useState(true)
    const [isOnline, setIsOnline] = useState(false)
    const [providerData, setProviderData] = useState(null)

    useEffect(() => {
        console.log('inside use effect')
        if (infoSocket) {
            infoSocket.close()
        }
        infoSocket = new WebSocket(`ws://${window.location.host}/admin/provider/${nickname}/info-ws`);

        infoSocket.onerror = (error) => {
            setIsOnline(false)
            setIsLoading(false)
            setProviderData([])
            setDevicesData(null)
        };

        infoSocket.onclose = () => {
            setIsOnline(false)
            setIsLoading(false)
            setProviderData([])
            setDevicesData(null)
        }

        infoSocket.onmessage = (message) => {
            if (isLoading) {
                setIsLoading(false)
            }
            if (!isOnline) {
                setIsOnline(true)
            }
            let providerJSON = JSON.parse(message.data)
            let unixTimestamp = new Date().getTime();
            let diff = unixTimestamp - providerJSON.last_updated
            if (diff > 3000) {
                setIsOnline(false)
            }
            setProviderData(providerJSON)
            setDevicesData(providerJSON.provided_devices)
        }

        return () => {
            if (infoSocket) {
                console.log('info socket unmounted')
                infoSocket.close()
            }
        }
    }, [])

    if (isLoading) {
        return (
            <Skeleton variant="rounded" style={{ marginLeft: '10px', background: 'gray', animationDuration: '1s', width: '60%', height: '600px' }} />
        )
    } else {
        return (
            <Box>
                <InfoBox os={os} isOnline={isOnline}></InfoBox>
                <ConnectedDevices connectedDevices={providerData.connected_devices} isOnline={isOnline}></ConnectedDevices>
                <ProviderDevices devicesData={devicesData} isOnline={isOnline}></ProviderDevices>
            </Box >
        )
    }
}

function ConnectedDevices({ connectedDevices, isOnline }) {
    if (!isOnline) {
        return (
            <div style={{ height: '200px', width: '400px', backgroundColor: 'white', borderRadius: '10px', justifyContent: 'center', alignItems: 'center', display: 'flex', fontSize: '20px' }}>Provider offline</div>
        )
    } else {
        return (
            <Stack>
                <div>Connected devices</div>
                {connectedDevices.map((connectedDevice) => {
                    return (
                        <ConnectedDevice deviceInfo={connectedDevice}></ConnectedDevice>
                    )
                })
                }
            </Stack>
        )
    }
}

function ConnectedDevice({ deviceInfo }) {
    let img_src = deviceInfo.os === 'android' ? './images/android-logo.png' : './images/apple-logo.png'

    function handleClick() {

    }


    return (
        <Stack>
            <img src={img_src} style={{ width: '20px', height: '20px' }}></img>
            <div>{deviceInfo.udid}</div>
            <Button variant='contained' disabled={deviceInfo.is_configured} onClick={handleClick}>Configure</Button>
        </Stack>
    )
}

function ProviderDevices({ devicesData, isOnline }) {
    if (!isOnline) {
        return (
            <div style={{ height: '800px', width: '400px', backgroundColor: 'white', borderRadius: '10px', justifyContent: 'center', alignItems: 'center', display: 'flex', fontSize: '20px' }}>Provider offline</div>
        )
    } else {
        return (
            <>
                <Stack spacing={1} style={{ height: '800px', overflowY: 'scroll', backgroundColor: 'white', borderRadius: '5px' }}>
                    {devicesData.map((device) => {
                        return (
                            <ProviderDevice deviceInfo={device}>

                            </ProviderDevice>
                        )
                    })
                    }
                </Stack>
            </>
        )
    }
}