import { useParams } from 'react-router-dom'
import { useNavigate } from 'react-router-dom'
import StreamCanvas from './StreamCanvas/StreamCanvas.js'
import { Skeleton, Stack, Tooltip } from '@mui/material'
import { Button } from '@mui/material'
import TabularControl from './Tabs/TabularControl'
import { useContext, useEffect, useState } from 'react'
import { Auth } from '../../contexts/Auth'
import { api } from '../../services/api.js'
import { useDialog } from '../../contexts/DialogContext.js'
import { LoadingOverlayProvider, useLoadingOverlay } from '../../contexts/LoadingOverlayContext.js'

export default function DeviceControl() {
    const { userName } = useContext(Auth)
    const { showLoadingOverlay, hideLoadingOverlay } = useLoadingOverlay()
    const { udid } = useParams()
    const navigate = useNavigate()
    const [deviceData, setDeviceData] = useState(null)
    const [isLoading, setIsLoading] = useState(true)
    const [shouldShowStream, setShouldShowStream] = useState(true)
    let screenRatio = window.innerHeight / window.innerWidth

    const healthUrl = `/device/${udid}/health`
    let infoUrl = `/device/${udid}/info`

    let in_use_socket = null
    useEffect(() => {
        api.get(healthUrl)
            .then(() => {
                return api.get(infoUrl)
            })
            .then(response => {
                setDeviceData(response.data)
                setInterval(() => {
                    setIsLoading(false)
                }, 1000)
            })
            .catch(error => {
            })

        if (in_use_socket) {
            in_use_socket.close()
        }
        const protocol = window.location.protocol
        let wsType = 'ws'
        if (protocol === 'https') {
            wsType = 'wss'
        }
        // let socketUrl = `${wsType}://${window.location.host}/devices/control/${udid}/in-use`
        let socketUrl = `${wsType}://192.168.1.41:10000/devices/control/${udid}/in-use`
        in_use_socket = new WebSocket(socketUrl)
        in_use_socket.onopen = () => {
            console.log('In Use WebSocket connection opened')
        }

        in_use_socket.onclose = () => {
            console.log('In Use WebSocket connection closed')
        }

        in_use_socket.onerror = (error) => {
            console.error('In Use WebSocket error:', error)
        }

        in_use_socket.onmessage = (event) => {
            if (in_use_socket.readyState === WebSocket.OPEN) {
                const message = JSON.parse(event.data)
                switch (message.type) {
                    case 'ping':
                        in_use_socket.send(userName)
                        break
                    case 'releaseDevice':
                        setShouldShowStream(false)
                        openDeviceForciblyReleasedAlert()
                        if (in_use_socket) {
                            in_use_socket.close()
                        }
                        break
                }
            }
        }

        return () => {
            if (in_use_socket) {
                in_use_socket.close()
            }
        }

    }, [])

    const handleBackClick = () => {
        navigate('/devices')
    }

    const { showDialog } = useDialog()
    const openDeviceForciblyReleasedAlert = () => {
        function backToDevices() {
            navigate('/devices')
        }

        showDialog('deviceReleasedAlert', {
            title: 'Session terminated!',
            content: `You've been kicked out by admin.`,
            actions: [
                { label: 'Back to devices', onClick: () => backToDevices() },
            ],
            isCloseable: false
        })
    }

    const refreshAppiumSession = () => {
        showLoadingOverlay()
        api.get(healthUrl)
            .then(() => {
                return api.get(infoUrl)
            })
            .then(response => {
                setDeviceData(response.data)
                setInterval(() => {
                    setIsLoading(false)
                }, 1000)
            })
            .catch(() => {
            })
            .finally(() => {
                hideLoadingOverlay()
            })
    }



    return (
        <div>
            <div className='back-button-bar' style={{
                marginBottom: '10px',
                marginTop: '10px'
            }}>
                <Button
                    variant='contained'
                    onClick={handleBackClick}
                    style={{
                        marginLeft: '20px',
                        backgroundColor: '#2f3b26',
                        color: '#9ba984',
                        fontWeight: 'bold'
                    }}
                >Back to devices</Button>
                <Tooltip
                    title='Refresh the Appium session'
                    arrow
                    placement='bottom'
                >
                    <Button
                        onClick={refreshAppiumSession}
                        startIcon={
                            <img
                                src="/images/appium-logo.png"
                                alt="icon"
                                style={{
                                    width: '24px',
                                    height: '24px',
                                }}
                            />
                        }
                        variant='contained'
                        style={{
                            marginLeft: '20px',
                            backgroundColor: '#2f3b26',
                            color: '#9ba984',
                            fontWeight: 'bold'
                        }}
                    >
                        Refresh
                    </Button>
                </Tooltip>
            </div>
            {
                isLoading ? (
                    <Stack
                        direction='row'
                        spacing={2}
                        style={{
                            marginLeft: '20px'
                        }}
                    >
                        <Skeleton
                            variant='rounded'
                            style={{
                                backgroundColor: 'gray',
                                animationDuration: '1s',
                                height: (window.innerHeight * 0.7),
                                width: (window.innerHeight * 0.7) * screenRatio,
                                borderRadius: '30px'
                            }}
                        />
                        <Skeleton
                            variant='rounded'
                            style={{
                                backgroundColor: 'gray',
                                animationDuration: '1s',
                                height: (window.innerHeight * 0.7),
                                width: '100%',
                                marginRight: '10px'
                            }}
                        />
                    </Stack>
                ) : (
                    <>
                        <Stack
                            direction='row'
                            spacing={2}
                            style={{
                                marginLeft: '20px'
                            }}
                        >
                            <StreamCanvas
                                deviceData={deviceData}
                                shouldShowStream={shouldShowStream}
                            />
                            <TabularControl
                                deviceData={deviceData}
                            ></TabularControl>
                        </Stack>
                    </>
                )
            }
        </div>
    )
}