import { Auth } from "../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import Provider from "./Provider/Provider";
import axios from "axios";
import Skeleton from '@mui/material/Skeleton';
import { Stack } from "@mui/material";


export default function ProvidersAdministration() {
    const [authToken, , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [providerInfo, setProviderInfo] = useState()
    const [isLoading, setIsLoading] = useState(true)

    let url = `/admin/providers`

    useEffect(() => {
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                setProviders(response.data)
                setProviderInfo(response.data[0])
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

        setInterval(() => {
            setIsLoading(false)
        }, 1500);

    }, [])

    return (
        <Stack spacing={2} width='230px' height='80%' style={{ overflowY: 'scroll', marginTop: '10px' }}>
            {
                isLoading ? (
                    <Skeleton variant="rounded" style={{ marginLeft: '10px', background: '#496612', animationDuration: '1s', height: '100%' }} />
                ) : (
                    <>
                        {providers.map((provider) => (
                            <Provider info={provider}></Provider>
                        ))}
                    </>
                )
            }
        </Stack>
    )
}