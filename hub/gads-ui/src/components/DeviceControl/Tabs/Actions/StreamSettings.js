import { Box, Button, CircularProgress, MenuItem, Select, Stack, TextField } from "@mui/material"
import { useState } from "react"
import { api } from "../../../../services/api"
import CheckIcon from "@mui/icons-material/Check"
import CloseIcon from "@mui/icons-material/Close"

export default function StreamSettings({ deviceData }) {
    const [fps, setFps] = useState(0)
    const [jpegQuality, setJpegQuality] = useState(0)
    const [scalingFactor, setScalingFactor] = useState(0)
    const [loading, setLoading] = useState(false)
    const [updateSettingsStatus, setUpdateSettingsStatus] = useState(null)

    const fpsOptions = [5, 10, 15, 20, 30, 45, 60];
    const jpegQualityOptions = [25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90]
    const scalingFactorOptions = [25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100]

    function buildPayload() {
        let body = {}
        body.target_fps = fps
        body.jpeg_quality = jpegQuality
        body.scaling_factor = scalingFactor


        let bodyString = JSON.stringify(body)
        return bodyString
    }

    function updateStreamSettings(event) {
        setLoading(true)
        setUpdateSettingsStatus(null)
        event.preventDefault()

        let url = `/device/${deviceData.udid}/update-stream-settings`
        let bodyString = buildPayload()

        api.post(url, bodyString, {})
            .then(() => {
                setUpdateSettingsStatus('success')
            })
            .catch(() => {
                setUpdateSettingsStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    setUpdateSettingsStatus(null)
                }, 1000)
            })
    }

    return (
        <Box
            marginTop='10px'
            style={{
                backgroundColor: "#9ba984",
                width: "250px"
            }}
        >
            <Stack
                spacing={2}
                style={{
                    marginTop: '10px',
                    marginLeft: "10px",
                    marginBottom: "10px",
                    marginRight: "10px"
                }}
            >
                <TextField
                    variant='outlined'
                    select
                    size='small'
                    label='Target FPS'
                    value={fps}
                    onChange={(event) => setFps(event.target.value)}
                    SelectProps={{
                        MenuProps: {
                            PaperProps: {
                                style: {
                                    maxHeight: 200,
                                },
                            },
                        },
                    }}
                >
                    {fpsOptions.map((option) => (
                        <MenuItem key={option} value={option}>
                            {option} FPS
                        </MenuItem>
                    ))}
                </TextField>
                <TextField
                    variant='outlined'
                    select
                    size='small'
                    label='JPEG quality'
                    value={jpegQuality}
                    onChange={(event) => setJpegQuality(event.target.value)}
                    SelectProps={{
                        MenuProps: {
                            PaperProps: {
                                style: {
                                    maxHeight: 200,
                                },
                            },
                        },
                    }}
                >
                    {jpegQualityOptions.map((option) => (
                        <MenuItem key={option} value={option}>
                            {option}
                        </MenuItem>
                    ))}
                </TextField>
                <TextField
                    variant='outlined'
                    select
                    size='small'
                    label='Scaling factor'
                    value={scalingFactor}
                    onChange={(event) => setScalingFactor(event.target.value)}
                    SelectProps={{
                        MenuProps: {
                            PaperProps: {
                                style: {
                                    maxHeight: 200,
                                },
                            },
                        },
                    }}
                >
                    {scalingFactorOptions.map((option) => (
                        <MenuItem key={option} value={option}>
                            {option}%
                        </MenuItem>
                    ))}
                </TextField>
                <Button
                    id='clipboard-button'
                    variant='contained'
                    style={{
                        color: "#9ba984",
                        fontWeight: "bold"
                    }}
                    onClick={updateStreamSettings}
                >
                    {loading ? (
                        <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                    ) : updateSettingsStatus === 'success' ? (
                        <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                    ) : updateSettingsStatus === 'error' ? (
                        <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                    ) : (
                        'Update settings'
                    )}</Button>
            </Stack>
        </Box>
    )
}