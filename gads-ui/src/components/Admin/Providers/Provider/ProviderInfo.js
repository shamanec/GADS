import { Box, Stack } from "@mui/material"
import { useEffect, useState } from "react"

export default function ProviderInfo({ info, isOnline }) {
    const [statusColor, setStatusColor] = useState('')
    const [status, setStatus] = useState('')

    useEffect(() => {
        if (isOnline) {
            setStatus('Online')
            setStatusColor('green')
        } else {
            setStatus('Offline')
            setStatusColor('red')
        }
    }, [isOnline])

    return (
        <Box display='flex' style={{ height: '40px', width: "200px", backgroundColor: 'white', alignItems: 'center', justifyContent: 'center' }}>
            <Stack direction='row' spacing={1}>
                <div>Status</div>
                <div style={{ width: '20px', height: '20px', borderRadius: '50%', backgroundColor: `${statusColor}` }}></div>
            </Stack>
        </Box>
    )
}