import { Auth } from "../../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import Providers from "./Providers";
import Skeleton from '@mui/material/Skeleton';
import { Box, Stack } from "@mui/material";
import { api } from '../../../services/api.js'

export default function ProvidersAdministration() {
    const [authToken, , , , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [isLoading, setIsLoading] = useState(true)
    let url = `/admin/providers`

    useEffect(() => {
        api.get(url, {})
            .then((response) => {
                setProviders(response.data)
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

        setTimeout(() => {
            setIsLoading(false)
        }, 1500);

    }, [])

    return (
        <Box
            height='100%'
            width="100%"
        >
            <Stack
                direction='row'
                height='80%'
                spacing={1}
            >
                {
                    isLoading ? (
                        <Skeleton
                            variant="rounded"
                            style={{
                                margin: '10px',
                                background: '#878a91',
                                animationDuration: '1s',
                                height: '100%',
                                width: '100%'
                            }} />
                    ) : (
                        <Providers
                            providers={providers}
                            setProviders={setProviders}
                        ></Providers>
                    )
                }
            </Stack>

        </Box>

    )
}