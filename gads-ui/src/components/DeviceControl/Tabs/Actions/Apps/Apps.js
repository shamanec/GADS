import { Box, Divider, Stack } from "@mui/material"
import { useState } from "react"
import UploadFile from "./UploadAppFile"
import InstallApp from "./InstallApp"

export default function Apps({ deviceData }) {
    const [installableApps, setInstallableApps] = useState([])

    return (
        <Box style={{ width: '600px', backgroundColor: 'white', borderRadius: '10px' }}>
            <Stack direction='row'>
                <UploadFile deviceData={deviceData} setInstallableApps={setInstallableApps}>

                </UploadFile>
                <Divider orientation="vertical" flexItem />
                <InstallApp installableApps={installableApps}>

                </InstallApp>
            </Stack>
        </Box>
    )
}