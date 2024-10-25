import { useParams } from 'react-router-dom'
import { useNavigate } from 'react-router-dom'
import StreamCanvas from './StreamCanvas/StreamCanvas.js'
import { Skeleton, Stack } from '@mui/material'
import { Button } from '@mui/material'
import TabularControl from './Tabs/TabularControl'
import { useContext, useEffect, useState } from 'react'
import { Auth } from '../../contexts/Auth'
import { DialogProvider } from './SessionDialogContext'
import { api } from '../../services/api.js'

export default function DeviceControl() {
    const { logout, userName } = useContext(Auth)
    const { udid } = useParams()
    const navigate = useNavigate()
    const [deviceData, setDeviceData] = useState(null)
    const [isLoading, setIsLoading] = useState(true)
    let screenRatio = window.innerHeight / window.innerWidth

    const healthUrl = `/device/${udid}/health`
    let infoUrl = `/device/${udid}/info`

    let in_use_socket = null
    useEffect(() => {
        api.get(healthUrl)
            .then((response) => {
                return api.get(infoUrl)
            })
            .then(response => {
                setDeviceData(response.data)
                setInterval(() => {
                    setIsLoading(false)
                }, 1000);
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                }
                // navigate('/devices')
            })

        if (in_use_socket) {
            in_use_socket.close()
        }
        const protocol = window.location.protocol;
        let wsType = "ws"
        if (protocol === "https") {
            wsType = "wss"
        }
        let socketUrl = `${wsType}://${window.location.host}/devices/control/${udid}/in-use`
        // let socketUrl = `${wsType}://192.168.1.41:10000/devices/control/${udid}/in-use`
        in_use_socket = new WebSocket(socketUrl)
        in_use_socket.onopen = () => {
            console.log('In Use WebSocket connection opened');
        };

        in_use_socket.onclose = () => {
            console.log('In Use WebSocket connection closed');
        };

        in_use_socket.onerror = (error) => {
            console.error('In Use WebSocket error:', error);
        };

        in_use_socket.onmessage = (message) => {
            if (in_use_socket.readyState === WebSocket.OPEN) {
                in_use_socket.send(userName)
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
    };

    return (
        <DialogProvider>
            <div>
                <div className='back-button-bar' style={{
                    marginBottom: '10px',
                    marginTop: '10px'
                }}>
                    <Button
                        variant="contained"
                        onClick={handleBackClick}
                        style={{
                            marginLeft: "20px",
                            backgroundColor: "#2f3b26",
                            color: "#9ba984",
                            fontWeight: "bold"
                        }}
                    >Back to devices</Button>
                </div>
                {
                    isLoading ? (
                        <Stack
                            direction='row'
                            spacing={2}
                            style={{
                                marginLeft: "20px"
                            }}
                        >
                            <Skeleton
                                variant="rounded"
                                style={{
                                    backgroundColor: 'gray',
                                    animationDuration: '1s',
                                    height: (window.innerHeight * 0.7),
                                    width: (window.innerHeight * 0.7) * screenRatio,
                                    borderRadius: '30px'
                                }}
                            />
                            <Skeleton
                                variant="rounded"
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
                                    marginLeft: "20px"
                                }}
                            >
                                <StreamCanvas
                                    deviceData={deviceData}
                                ></StreamCanvas>
                                <TabularControl
                                    deviceData={deviceData}
                                ></TabularControl>
                            </Stack>
                        </>
                    )
                }
            </div>
        </DialogProvider>
    )
}