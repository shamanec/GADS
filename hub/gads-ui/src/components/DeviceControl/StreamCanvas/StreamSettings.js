import { Box, Button, CircularProgress, MenuItem, Stack, TextField } from '@mui/material'
import { useState } from 'react'
import { api } from '../../../services/api'
import CheckIcon from '@mui/icons-material/Check'
import CloseIcon from '@mui/icons-material/Close'

export default function StreamSettings({ deviceData }) {
    const [fps, setFps] = useState(deviceData.stream_target_fps)
    const [jpegQuality, setJpegQuality] = useState(deviceData.stream_jpeg_quality)
    const [scalingFactor, setScalingFactor] = useState(deviceData.stream_scaling_factor)
    const [isLoading, setIsLoading] = useState(false)
    const [updateSettingsStatus, setUpdateSettingsStatus] = useState(null)

    const fpsOptions = [5, 10, 15, 20, 30, 45, 60]
    const jpegQualityOptions = [25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90]
    const scalingFactorOptionsiOS = [25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100]
    const scalingFactorOptionsAndroid = [25, 50]

    const scalingFactorOptions = deviceData.os === 'ios' ? scalingFactorOptionsiOS : scalingFactorOptionsAndroid

    function buildPayload() {
        let body = {}
        body.target_fps = fps
        body.jpeg_quality = jpegQuality
        body.scaling_factor = scalingFactor


        let bodyString = JSON.stringify(body)
        return bodyString
    }

    function updateStreamSettings(event) {
        setIsLoading(true)
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
                    setIsLoading(false)
                    setTimeout(() => {
                        setUpdateSettingsStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    return (
        <Box
            style={{
                backgroundColor: '#9ba984',
                width: '100%',
                height: '250px',
                alignContent: 'center',
                borderRadius: '5px'
            }}
        >
            <Stack
                spacing={2}
                style={{
                    marginTop: '10px',
                    marginLeft: '10px',
                    marginBottom: '10px',
                    marginRight: '10px'
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
                    variant='contained'
                    style={{
                        backgroundColor: isLoading ? 'rgba(51,71,110,0.47)' : '#2f3b26',
                        color: '#9ba984',
                        fontWeight: 'bold'
                    }}
                    onClick={updateStreamSettings}
                >
                    {isLoading ? (
                        <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                    ) : updateSettingsStatus === 'success' ? (
                        <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                    ) : updateSettingsStatus === 'error' ? (
                        <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                    ) : (
                        'Update'
                    )}</Button>
            </Stack>
        </Box>
    )
}