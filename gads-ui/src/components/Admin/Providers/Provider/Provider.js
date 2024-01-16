import { Box, Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import { useEffect, useState } from "react";
import ProviderInfo from "./ProviderInfo";
import ProviderDevice from "./ProviderDevice"

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
                <ProviderDevices
                    devicesData={devicesData}
                    isOnline={isOnline}
                ></ProviderDevices>
            </Box >
        )
    }
}

function ProviderDevices({ devicesData, isOnline }) {
    if (!isOnline || devicesData === null) {
        return (
            <div
                style={{
                    height: '600px',
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
                        height: '600px',
                        overflowY: 'scroll',
                        backgroundColor: 'white',
                        borderRadius: '5px',
                        overflowY: 'scroll',
                        marginTop: '10px'
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