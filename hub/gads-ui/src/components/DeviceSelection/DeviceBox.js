import { Box, Stack, List, ListItemIcon, ListItem, ListItemText, Divider, Button, CircularProgress } from '@mui/material'
import HomeIcon from '@mui/icons-material/Home'
import InfoIcon from '@mui/icons-material/Info'
import AspectRatioIcon from '@mui/icons-material/AspectRatio'
import PhoneAndroidIcon from '@mui/icons-material/PhoneAndroid'
import PhoneIphoneIcon from '@mui/icons-material/PhoneIphone'
import { useNavigate } from 'react-router-dom'
import React, { useEffect, useState } from 'react'
import './DeviceBox.css'
import { api } from '../../services/api'
import { useSnackbar } from '../../contexts/SnackBarContext'

export default function DeviceBox({ device }) {
    const [isAdmin, setIsAdmin] = useState(false)
    let img_src = device.info.os === 'android' ? './images/android-logo.png' : './images/apple-logo.png'

    useEffect(() => {
        let roleFromStorage = localStorage.getItem('userRole')
        if (roleFromStorage === 'admin') {
            setIsAdmin(true)
        }
    }, [])

    return (
        <Box
            className='device-box'
        >
            <Stack
                divider={<Divider orientation='horizontal' flexItem />}
            >
                <Box
                    className='status-box'
                >
                    <Stack
                        direction='row'
                        spacing={1}
                        alignItems='center'
                    >
                        <Box
                            className='logo-box'
                        >
                            <img
                                src={img_src}
                                height='50px'
                            >
                            </img>
                        </Box>
                        <DeviceStatus device={device}
                        ></DeviceStatus>

                    </Stack>
                </Box>
                <Box className='info-box'>
                    <List
                        id='info-list'
                        dense='true'
                    >
                        <ListItem>
                            <ListItemIcon>
                                {device.info.os === 'ios' ? (
                                    <PhoneIphoneIcon></PhoneIphoneIcon>
                                ) : (
                                    <PhoneAndroidIcon></PhoneAndroidIcon>
                                )}
                            </ListItemIcon>
                            <ListItemText
                                className='filterable info-text'
                                primary={device.info.name}
                            />
                        </ListItem>
                        <ListItem>
                            <ListItemIcon>
                                <InfoIcon />
                            </ListItemIcon>
                            <ListItemText
                                className='filterable info-text'
                                primary={device.info.udid}
                            />
                        </ListItem>
                        <ListItem>
                            <ListItemIcon>
                                <AspectRatioIcon />
                            </ListItemIcon>
                            <ListItemText
                                className='filterable info-text'
                                primary={device.info.screen_width + 'x' + device.info.screen_height}
                            />
                        </ListItem>
                        <ListItem>
                            <ListItemIcon>
                                <InfoIcon />
                            </ListItemIcon>
                            <ListItemText
                                className='filterable info-text'
                                primary={device.info.os_version}
                            />
                        </ListItem>
                        <ListItem>
                            <ListItemIcon>
                                <HomeIcon />
                            </ListItemIcon>
                            <ListItemText
                                className='filterable info-text'
                                primary={device.info.provider}
                            />
                        </ListItem>
                    </List>
                </Box>
                <Stack
                    direction='row'
                    spacing={1}
                    justifyContent='flex-end'
                    alignItems='center'
                    alignContent='center'
                    height='60px'
                    marginRight='10px'
                >
                    <ReleaseButton
                        device={device}
                        isAdmin={isAdmin}
                    ></ReleaseButton>
                    <UseButton device={device}></UseButton>
                </Stack>
            </Stack>
        </Box>
    )
}

function DeviceStatus({ device }) {
    if (device.info.usage === 'disabled') {
        return (
            <div
                className='offline-status'
            >Disabled</div>
        )
    }

    if (device.available) {
        if (device.info.usage === 'automation') {
            if (device.is_running_automation) {
                return (
                    <div>
                        <div
                            className='automation-status'
                        >Running automation</div>
                    </div>
                )
            } else {
                return (
                    <div
                        className='automation-status'
                    >Automation only</div>
                )
            }
        }

        if (device.info.usage === 'enabled' || device.info.usage === 'control') {
            if (device.is_running_automation) {
                return (
                    <div>
                        <div
                            className='automation-status'
                        >Running automation</div>
                    </div>
                )
            }
            if (device.in_use === true) {
                return (
                    <div className='in-use-status'>
                        <div style={{ textDecoration: 'underline' }}>Currently in use</div>
                        <div style={{ marginTop: '5px' }}>{device.in_use_by}</div>
                    </div>
                )
            } else {
                return (
                    <div
                        className='available-status'
                    >Available</div>
                )
            }
        }
    } else {
        return (
            <div
                className='offline-status'
            >Offline</div>
        )
    }
}

function ReleaseButton({ device, isAdmin }) {
    const { showSnackbar } = useSnackbar()
    const [releasing, setReleasing] = useState(false)

    const showCustomSnackbarMessage = (message, severity) => {
        showSnackbar({
            message: message,
            severity: severity,
            duration: 3000,
        })
    }

    function handleReleaseButtonClick() {
        setReleasing(true)

        api.post(`/admin/device/${device.info.udid}/release`)
            .then(() => {
                showCustomSnackbarMessage('Device released!', 'success')
            })
            .catch(() => {
                showCustomSnackbarMessage('Failed to release device!', 'error')
            })
            .finally(() => {
                setTimeout(() => {
                    setReleasing(false)
                }, 1000)
            })
    }


    return (
        <Button
            onClick={handleReleaseButtonClick}
            variant='contained'
            disabled={!device.in_use || releasing}
            style={{
                backgroundColor: (!device.in_use || releasing) ? '#878a91' : '#2f3b26',
                color: '#f4e6cd',
                fontWeight: 'bold',
                boxShadow: 'none',
                height: '40px',
                width: '100px',
                display: isAdmin ? 'block' : 'none'
            }}
        >
            {releasing ? (
                <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
            ) : (
                'Release'
            )}
        </Button>
    )
}

function UseButton({ device }) {
    const [loading, setLoading] = useState(false)
    const navigate = useNavigate()

    function handleUseButtonClick() {
        setLoading(true)
        setTimeout(() => {
            navigate('/devices/control/' + device.info.udid, device)
        }, 1000)
    }

    const buttonDisabled = loading || !device.info.connected

    if (device.info.usage === 'disabled') {
        return (
            <button
                className='device-buttons'
                variant='contained'
                disabled
            >
                N/A
            </button>
        )
    }

    if (device.available) {
        if (device.info.usage === 'automation') {
            return (
                <button
                    className='device-buttons'
                    variant='contained'
                    disabled
                >
                    N/A
                </button>
            )
        }
        if (device.is_running_automation || device.in_use) {
            return (
                <button
                    className='device-buttons'
                    disabled
                >
                    In Use
                </button>
            )
        } else {
            return (
                <button
                    className='device-buttons'
                    onClick={handleUseButtonClick}
                    disabled={buttonDisabled}
                >
                    {loading ? <span className='spinner'></span> : 'Use'}
                </button>
            )
        }

    } else {
        return (
            <button
                className='device-buttons'
                variant='contained'
                disabled
            >
                N/A
            </button>
        )
    }
}