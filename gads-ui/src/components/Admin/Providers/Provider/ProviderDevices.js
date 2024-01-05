import { Stack } from "@mui/material"
import ProviderDevice from "./ProviderDevice"

export default function ProviderDevices({ devicesData }) {
    // const [isLoading, setIsLoading] = useState(true)

    return (
        <>
            <Stack spacing={1} style={{ height: '800px', overflowY: 'scroll', backgroundColor: 'white', borderRadius: '5px' }}>
                {devicesData.map((device) => {
                    return (
                        <ProviderDevice deviceInfo={device}>

                        </ProviderDevice>
                    )
                })
                }
            </Stack>

        </>
    )
}