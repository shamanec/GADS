import { Box, Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import { useEffect, useState } from "react";
import ProviderInfo from "./ProviderInfo";
import ProviderDevice from "./ProviderDevice"
import ProviderLogsTable from "./ProviderLogsTable/ProviderLogsTable";

export default function Provider({ info }) {
    return (
        <Stack
            width="100%"
            style={{
                marginTop: '10px',
                marginBottom: '10px',
                borderRadius: '10px',
                padding: '10px'
            }}
        >
            <Stack
                width="100%"
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
                <ProviderLogsTable
                    nickname={info.nickname}
                >
                </ProviderLogsTable>
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
        // Use specific full address for local development, proxy does not seem to work okay
        const evtSource = new EventSource(`http://192.168.1.6:10000/admin/provider/${nickname}/info`);
        // const evtSource = new EventSource(`/admin/provider/${nickname}/info`);

        evtSource.onmessage = (event) => {
            let providerJSON = JSON.parse(event.data)
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
            evtSource.close()
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
                    backgroundColor: '#78866B',
                    borderRadius: '10px',
                    textAlign: 'center',
                    fontSize: '20px',
                    display: 'flex',
                    justifyContent: 'center',
                    alignItems: 'center',
                    marginTop: '10px'
                }}
            >No device data or provider offline</div>
        )
    } else {
        return (
            <>
                <Stack
                    spacing={1}
                    style={{
                        height: '600px',
                        overflowY: 'scroll',
                        backgroundColor: '#78866B',
                        borderRadius: '5px',
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