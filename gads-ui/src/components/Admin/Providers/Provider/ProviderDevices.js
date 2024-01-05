import { Stack } from "@mui/material"
import { useState } from "react"

export default function ProviderDevices({ devicesData }) {
    // const [isLoading, setIsLoading] = useState(true)

    return (
        <>
            <Stack>
                {devicesData.map((device) => {
                    return (
                        <div>{device.Device.udid}</div>
                    )
                })
                }
            </Stack>

        </>
    )
}