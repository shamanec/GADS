import { Skeleton, Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import ProviderDevices from "./ProviderDevices";
import { useContext, useEffect, useState } from "react";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";

export default function Provider({ info }) {
    const [authToken, , logout] = useContext(Auth)
    const [devicesData, setDevicesData] = useState([])
    const [isLoading, setIsLoading] = useState(true)
    console.log('in provider')
    console.log(info)

    useEffect(() => {
        console.log('use effect')
        let url = `/provider/${info.nickname}/info`
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).then((response) => {
            setDevicesData(response.data.device_data)
        })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                }
                console.log('Failed getting providers data' + error)
                return
            })
        setTimeout(() => {
            setIsLoading(false)
        }, 1000)
    }, [])

    return (
        <Stack direction='row'>
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
    )
}