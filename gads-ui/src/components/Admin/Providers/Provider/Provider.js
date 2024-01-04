import { Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";
import ProviderDevices from "./ProviderDevices";
import { useContext, useEffect, useState } from "react";
import axios from "axios";
import { Auth } from "../../../../contexts/Auth";

export default function Provider({ isNew, data }) {
    const [authToken, , logout] = useContext(Auth)
    const [devicesData, setDevicesData] = useState([])

    useEffect(() => {
        let url = `/provider/${data.nickname}/info`
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
            });
    }, [])

    return (
        <Stack direction='row'>
            <ProviderConfig isNew={isNew} data={data}>
            </ProviderConfig>
            <ProviderDevices devicesData={devicesData}></ProviderDevices>
        </Stack>
    )
}