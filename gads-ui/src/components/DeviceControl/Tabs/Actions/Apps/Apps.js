import { Box, Divider, Stack } from "@mui/material"
import { useState } from "react"
import UploadFile from "./UploadAppFile"
import InstallApp from "./InstallApp"

export default function Apps({ deviceData }) {
    const [installableApps, setInstallableApps] = useState(deviceData.installable_apps)

    return (
        <Box
            style={{
                width: '600px',
                backgroundColor: 'white',
                borderRadius: '10px'
            }}
        >
            <Stack
                direction='row'
            >
                <UploadFile
                    deviceData={deviceData}
                    setInstallableApps={setInstallableApps}
                >

                </UploadFile>
                <Divider
                    orientation="vertical"
                    flexItem
                />
                <InstallApp
                    udid={deviceData.udid}
                    installableApps={installableApps}
                    installedApps={deviceData.installed_apps}
                >
                </InstallApp>
            </Stack>
        </Box>
    )
}