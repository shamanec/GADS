import { Box, Stack } from "@mui/material"
import { useEffect, useState } from "react"

export default function ProviderInfo({ os, isOnline }) {
    const [statusColor, setStatusColor] = useState('')
    const [status, setStatus] = useState('')
    const logoPath = `./images/${os}-logo.png`

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
        <Box
            display='flex'
            style={{
                height: '50px',
                width: "200px",
                backgroundColor: 'white',
                alignItems: 'center',
                justifyContent: 'center',
                borderRadius: '10px'
            }}
        >
            <Status
                logoPath={logoPath}
                statusColor={statusColor}
            ></Status>
        </Box>
    )
}

function Status({ logoPath, statusColor }) {
    return (
        <Stack
            direction='row'
            spacing={1}
            alignItems='center'
        >
            <img
                src={logoPath}
                style={{
                    height: '40px',
                    width: '40px'
                }}
            ></img>
            <div>Status</div>
            <div
                style={{
                    width: '20px',
                    height: '20px',
                    borderRadius: '50%',
                    backgroundColor: `${statusColor}`
                }}
            ></div>
        </Stack>
    )
}