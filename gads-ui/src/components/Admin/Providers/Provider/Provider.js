import { Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import ProviderDevices from "./ProviderDevices";
import { useContext, useEffect, useState } from "react";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";
import ProviderInfo from "./ProviderInfo";

export default function Provider({ info }) {
    const [authToken, , , , logout] = useContext(Auth)
    const [devicesData, setDevicesData] = useState([])
    const [isLoading, setIsLoading] = useState(true)
    const [isOnline, setIsOnline] = useState(false)

    useEffect(() => {
        const axiosController = new AbortController();
        setDevicesData([])
        setIsOnline(false)
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
                return
            })
        setTimeout(() => {
            setIsLoading(false)
        }, 1000)

        return () => {
            axiosController.abort()
        }

    }, [info])

    return (
        <Stack id='koleo'>
            <ProviderInfo isOnline={isOnline}></ProviderInfo>
            <Stack direction='row' spacing={1}>
                <ProviderConfig isNew={false} data={info}>
                </ProviderConfig>
                {
                    isLoading ? (
                        <Skeleton variant="rounded" style={{ marginLeft: '10px', background: '#496612', animationDuration: '1s', height: '100%', width: '500px' }} />
                    ) : (
                        <ProviderDevices devicesData={devicesData}></ProviderDevices>
                    )
                }

            </Stack>
        </Stack>
    )
}