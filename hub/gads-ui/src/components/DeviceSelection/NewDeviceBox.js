import { Box, Stack, List, ListItemIcon, ListItem, ListItemText, Divider, Button } from "@mui/material";
import HomeIcon from '@mui/icons-material/Home';
import InfoIcon from '@mui/icons-material/Info';
import AspectRatioIcon from '@mui/icons-material/AspectRatio';
import PhoneAndroidIcon from '@mui/icons-material/PhoneAndroid';
import PhoneIphoneIcon from '@mui/icons-material/PhoneIphone';
import { api } from '../../services/api.js'
import { useNavigate } from 'react-router-dom';
import React, { useState } from 'react'
import './NewDeviceBox.css'

export default function NewDeviceBox({ device }) {
    let img_src = device.info.os === 'android' ? './images/android-logo.png' : './images/apple-logo.png'

    return (
        <Box
            className='device-box'
        >
            <Stack
                divider={<Divider orientation="horizontal" flexItem />}
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
                        <UseButton device={device}></UseButton>
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
            </Stack>
        </Box>
    )
}

function DeviceStatus({ device }) {
    if (device.available) {
        if (device.usage === "disabled") {
            return (
                <div
                    className='offline-status'
                >Disabled</div>
            )
        }
        if (device.info.usage === "automation") {
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
                        className='offline-status'
                    >Automation only</div>
                )
            }
        }

        if (device.info.usage === "enabled" || device.info.usage === "remote") {
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

function UseButton({ device }) {
    const [loading, setLoading] = useState(false)
    const navigate = useNavigate();

    function handleUseButtonClick() {
        setLoading(true);
        const url = `/device/${device.info.udid}/health`;
        api.get(url)
            .then(response => {
                if (response.status === 200) {
                    navigate('/devices/control/' + device.info.udid, device);
                }
            })
            .catch(() => {

            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false);
                }, 2000);
            });
    }

    const buttonDisabled = loading || !device.info.connected

    if (device.available) {
        if (device.is_running_automation || device.in_use) {
            return (
                <button
                    className='device-buttons'
                    disabled
                >In Use</button>
            )
        }
        return (
            <button
                className='device-buttons'
                onClick={handleUseButtonClick}
                disabled={buttonDisabled}
            >
                {loading ? <span className="spinner"></span> : 'Use'}
            </button>
        )
    } else {
        return (
            <button
                className='device-buttons'
                variant="contained"
                disabled
            >N/A</button>
        )
    }
}