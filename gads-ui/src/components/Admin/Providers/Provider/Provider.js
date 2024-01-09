import { Box, Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import ProviderDevices from "./ProviderDevices";
import { useContext, useEffect, useState } from "react";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";
import ProviderInfo from "./ProviderInfo";

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

    useEffect(() => {
        console.log('inside use effect')
        if (infoSocket) {
            infoSocket.close()
        }
        infoSocket = new WebSocket(`ws://${window.location.host}/admin/provider/${nickname}/info-ws`);

        infoSocket.onerror = (error) => {
            setIsOnline(false)
            setIsLoading(false)
            setDevicesData(null)
        };

        infoSocket.onclose = () => {
            setIsOnline(false)
            setIsLoading(false)
            setDevicesData(null)
        }

        infoSocket.onmessage = (message) => {
            if (!isOnline) {
                setIsOnline(true)
            }
            if (isLoading) {
                setIsLoading(false)
            }
            let providerJSON = JSON.parse(message.data)
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
                <ProviderDevices devicesData={devicesData}></ProviderDevices>
            </Box >
        )
    }
}