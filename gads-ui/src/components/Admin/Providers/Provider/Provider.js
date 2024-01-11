import { Box, Button, Divider, Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import { useContext, useEffect, useState } from "react";
import { Auth } from "../../../../contexts/Auth";
import ProviderInfo from "./ProviderInfo";
import ProviderDevice from "./ProviderDevice"
import axios from "axios";

export default function Provider({ info }) {
    return (
        <Stack
            style={{
                marginTop: '10px',
                marginBottom: '10px',
                borderRadius: '10px',
                padding: '10px'
            }}
        >
            <Stack
                direction='row'
                spacing={2}
            >
                <ProviderConfig
                    isNew={false}
                    data={info}
                >
                </ProviderConfig>
                <LiveProviderBox
                    nickname={info.nickname}
                    os={info.os}
                ></LiveProviderBox>
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
        };

        infoSocket.onclose = () => {
            setIsOnline(false)
            setIsLoading(false)
        }

        infoSocket.onmessage = (message) => {

            let providerJSON = JSON.parse(message.data)
            setProviderData(providerJSON)
            setDevicesData(providerJSON.provided_devices)

            let unixTimestamp = new Date().getTime();
            let diff = unixTimestamp - providerJSON.last_updated
            if (diff > 4000) {
                setIsOnline(false)
            } else {
                setIsOnline(true)
            }

            if (isLoading) {
                setIsLoading(false)
            }
        }

        return () => {
            if (infoSocket) {
                infoSocket.close()
            }
        }
    }, [])

    if (isLoading) {
        return (
            <Skeleton
                variant="rounded"
                style={{
                    marginLeft: '10px',
                    background: 'gray',
                    animationDuration: '1s',
                    width: '60%',
                    height: '600px'
                }}
            />
        )
    } else {
        return (
            <Box>
                <InfoBox
                    os={os}
                    isOnline={isOnline}
                ></InfoBox>
                <ConnectedDevices
                    connectedDevices={providerData.connected_devices}
                    isOnline={isOnline}
                    providerName={nickname}
                ></ConnectedDevices>
                <ProviderDevices
                    devicesData={devicesData}
                    isOnline={isOnline}
                ></ProviderDevices>
            </Box >
        )
    }
}

function ConnectedDevices({ connectedDevices, isOnline, providerName }) {
    if (!isOnline) {
        return (
            <div
                style={{
                    height: '200px',
                    width: '400px',
                    backgroundColor: 'white',
                    borderRadius: '10px',
                    justifyContent: 'center',
                    alignItems: 'center',
                    display: 'flex',
                    fontSize: '20px',
                    marginTop: '10px'
                }}
            >Provider offline</div>
        )
    } else {
        return (
            <Stack
                spacing={1}
                style={{
                    backgroundColor: 'white',
                    marginTop: '10px',
                    marginBottom: '10px',
                    borderRadius: '5px',
                    height: '200px',
                    overflowY: 'scroll'
                }}
            >
                <div
                    style={{
                        textAlign: 'center',
                        marginTop: '5px'
                    }}
                >Connected devices</div>
                <Divider></Divider>
                {connectedDevices.map((connectedDevice) => {
                    return (
                        <ConnectedDevice
                            deviceInfo={connectedDevice}
                            providerName={providerName}
                        ></ConnectedDevice>
                    )
                })
                }
            </Stack>
        )
    }
}

function ConnectedDevice({ deviceInfo, providerName }) {
    const [authToken, , , , logout] = useContext(Auth)
    let img_src = deviceInfo.os === 'android' ? './images/android-logo.png' : './images/apple-logo.png'

    function handleClick() {
        let url = `/admin/devices/add`

        let body = {}
        body.udid = deviceInfo.udid
        body.provider = providerName
        body.os = deviceInfo.os
        if (deviceInfo.os === 'android') {
            body.name = 'Android'
        } else {
            body.name = 'iPhone'
        }
        let bodyString = JSON.stringify(body)

        axios.post(url, bodyString, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).catch((error) => {
            if (error.response) {
                if (error.response.status === 401) {
                    logout()
                    return
                }
                console.log(error.response)
            }
        })
    }


    return (
        <Stack>
            <img
                src={img_src}
                style={{
                    width: '20px',
                    height: '20px'
                }}>
            </img>
            <div>{deviceInfo.udid}</div>
            {deviceInfo.is_configured ? (
                <div style={{ color: 'green' }}>Device is in DB</div>
            ) : (
                <div style={{ color: 'red' }}>Device not configured in DB</div>
            )
            }
            <Button
                variant='contained'
                disabled={deviceInfo.is_configured}
                onClick={handleClick}
            >Configure</Button>
        </Stack>
    )
}

function ProviderDevices({ devicesData, isOnline }) {
    if (!isOnline || devicesData === null) {
        return (
            <div
                style={{
                    height: '400px',
                    width: '400px',
                    backgroundColor: 'white',
                    borderRadius: '10px',
                    textAlign: 'center',
                    fontSize: '20px',
                    display: 'flex',
                    justifyContent: 'center',
                    alignItems: 'center',
                    marginTop: '10px'
                }}
            >Provider offline or no local device data available yet</div>
        )
    } else {
        return (
            <>
                <Stack
                    spacing={1}
                    style={{
                        height: '400px',
                        overflowY: 'scroll',
                        backgroundColor: 'white',
                        borderRadius: '5px',
                        overflowY: 'scroll'
                    }}
                >
                    {devicesData.map((device) => {
                        return (
                            <ProviderDevice
                                deviceInfo={device}>
                            </ProviderDevice>
                        )
                    })
                    }
                </Stack>
            </>
        )
    }
}