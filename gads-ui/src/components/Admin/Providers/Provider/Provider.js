import { Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import ProviderDevices from "./ProviderDevices";
import { useContext, useEffect, useState } from "react";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";
import ProviderInfo from "./ProviderInfo";

export default function Provider({ info }) {
    const [authToken, , , , logout] = useContext(Auth)
    const [devicesData, setDevicesData] = useState('')
    const [isLoading, setIsLoading] = useState(true)
    const [isOnline, setIsOnline] = useState(false)

    useEffect(() => {
        const axiosController = new AbortController();
        setDevicesData(null)
        setIsOnline(false)
        setIsLoading(true)
        let url = `/provider/${info.nickname}/info`
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            },
            timeout: 5000,
            signal: axiosController.signal
        }).then((response) => {
            setDevicesData(response.data.device_data)
            setIsOnline(true)
        })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                }
            })
        setTimeout(() => {
            setIsLoading(false)
        }, 1000)

        return () => {
            axiosController.abort()
        }

    }, [info])

    function ProviderBox() {
        if (isLoading) {
            return (
                <Skeleton variant="rounded" style={{ marginLeft: '10px', background: 'gray', animationDuration: '1s', width: '60%', height: '600px' }} />
            )
        } else {
            return (
                <ProviderDevices devicesData={devicesData}></ProviderDevices>
            )
        }
    }

    return (
        <Stack id='koleo' style={{ marginTop: '10px', marginBottom: '10px', borderRadius: '10px', padding: '10px' }}>
            <ProviderInfo isOnline={isOnline}></ProviderInfo>
            <Stack direction='row' spacing={2}>
                <ProviderConfig
                    isNew={false}
                    data={info}
                >
                </ProviderConfig>
                <ProviderBox></ProviderBox>
            </Stack>
        </Stack>
    )
}