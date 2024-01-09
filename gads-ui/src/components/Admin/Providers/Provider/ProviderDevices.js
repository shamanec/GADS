import { Stack } from "@mui/material"
import ProviderDevice from "./ProviderDevice"

export default function ProviderDevices({ devicesData }) {
    if (devicesData === null) {
        return (
            <div style={{ height: '800px', width: '400px', backgroundColor: 'white', borderRadius: '10px', justifyContent: 'center', alignItems: 'center', display: 'flex', fontSize: '20px' }}>Provider offline, no device data available</div>
        )
    } else {
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
}