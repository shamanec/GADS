import { Box, Divider, Stack } from "@mui/material"
import { useState } from "react"
import UploadFile from "./UploadAppFile"
import UninstallApp from "./UninstallApp"

export default function Apps({ deviceData }) {
    return (
        <Box
            style={{
                width: '600px',
                backgroundColor: '#78866B',
                borderRadius: '10px'
            }}
        >
            <Stack
                direction='row'
            >
                <UploadFile
                    deviceData={deviceData}
                >

                </UploadFile>
                <Divider
                    orientation="vertical"
                    flexItem
                />
                <UninstallApp
                    udid={deviceData.udid}
                    installedApps={deviceData.installed_apps}
                >
                </UninstallApp>
            </Stack>
        </Box>
    )
}