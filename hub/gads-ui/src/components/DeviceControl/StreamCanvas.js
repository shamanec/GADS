import { Auth } from "../../contexts/Auth"
import { useContext, useEffect, useState } from "react"
import './StreamCanvas.css'
import { Button, Divider, Grid, Stack } from "@mui/material"
import HomeIcon from '@mui/icons-material/Home';
import LockOpenIcon from '@mui/icons-material/LockOpen';
import LockIcon from '@mui/icons-material/Lock';
import { useDialog } from "./SessionDialogContext"
import { api } from '../../services/api.js'

export default function StreamCanvas({ deviceData }) {
    const { authToken, logout } = useContext(Auth)
    const { setDialog } = useDialog()
    const [canvasSize, setCanvasSize] = useState({
        width: 0,
        height: 0
    });

    let deviceX = parseInt(deviceData.screen_width, 10)
    let deviceY = parseInt(deviceData.screen_height, 10)
    let screen_ratio = deviceX / deviceY
    // let canvasHeight = (window.innerHeight * 0.7)
    // let canvasWidth = (window.innerHeight * 0.7) * screen_ratio

    const streamData = {
        udid: deviceData.udid,
        deviceX: deviceX,
        deviceY: deviceY,
        screen_ratio: screen_ratio,
        canvasHeight: canvasSize.height,
        canvasWidth: canvasSize.width
    }

    let streamUrl = ""
    if (deviceData.os === 'ios') {
        // streamUrl = `http://192.168.1.6:10000/device/${deviceData.udid}/ios-stream-mjpeg`
        streamUrl = `/device/${deviceData.udid}/ios-stream-mjpeg`
    } else {
        // streamUrl = `http://192.168.1.6:10000/device/${deviceData.udid}/android-stream-mjpeg`
        streamUrl = `/device/${deviceData.udid}/android-stream-mjpeg`
    }

    useEffect(() => {
        const updateCanvasSize = () => {
            let canvasHeight = window.innerHeight * 0.7
            let canvasWidth = canvasHeight * screen_ratio

            setCanvasSize({
                width: canvasWidth,
                height: canvasHeight
            })
        }

        updateCanvasSize()

        // Set resize listener
        window.addEventListener('resize', updateCanvasSize);

        return () => {
            window.stop()
            window.removeEventListener('resize', updateCanvasSize);
        }
    }, []);

    return (
        <div
            id='phone-imitation'
        >
            <h3
                style={{
                    color: '#2f3b26',
                    display: 'flex',
                    fontFamily: 'Verdana',
                    justifyContent: 'center'
                }}
            >{deviceData.model}</h3>
            <div
                id="stream-div"
                style={{
                    width: streamData.canvasWidth,
                    height: streamData.canvasHeight
                }}
            >
                <Canvas
                    canvasWidth={streamData.canvasWidth}
                    canvasHeight={streamData.canvasHeight}
                    authToken={authToken}
                    logout={logout}
                    streamData={streamData}
                    setDialog={setDialog}
                ></Canvas>
                <Stream
                    canvasWidth={streamData.canvasWidth}
                    canvasHeight={streamData.canvasHeight}
                    streamUrl={streamUrl}
                ></Stream>
            </div>
            <Divider></Divider>
            <Grid
                height='50px'
                display='flex'
                justifyContent='center'
                style={{
                    marginTop: '10px'
                }}
            >
                <Button
                    onClick={() => homeButton(authToken, deviceData, setDialog)}
                    className='canvas-buttons'
                    startIcon={<HomeIcon />}
                    variant='contained'
                    style={{
                        fontWeight: "bold",
                        color: "#9ba984",
                        backgroundColor: "#2f3b26",
                        borderBottomLeftRadius: '25px',
                    }}
                >Home</Button>
                <Button
                    onClick={() => lockButton(authToken, deviceData, setDialog)}
                    className='canvas-buttons'
                    startIcon={<LockIcon />}
                    variant='contained'
                    style={{
                        fontWeight: "bold",
                        color: "#9ba984",
                        backgroundColor: "#2f3b26"
                    }}
                >Lock</Button>
                <Button
                    onClick={() => unlockButton(authToken, deviceData, setDialog)}
                    className='canvas-buttons'
                    startIcon={<LockOpenIcon />}
                    variant='contained'
                    style={{
                        fontWeight: "bold",
                        color: "#9ba984",
                        backgroundColor: "#2f3b26",
                        borderBottomRightRadius: '25px'
                    }}
                >Unlock</Button>
            </Grid>
        </div >

    )
}

function Canvas({ authToken, logout, streamData, setDialog }) {
    var tapStartAt = 0
    var coord1;
    var coord2;

    function getCursorCoordinates(event) {
        const rect = event.currentTarget.getBoundingClientRect()
        const x = event.clientX - rect.left
        const y = event.clientY - rect.top
        return [x, y];
    }

    function handleMouseDown(event) {
        tapStartAt = (new Date()).getTime()
        coord1 = getCursorCoordinates(event)
        console.log('Tapped on ' + coord1)
    }

    function handleMouseUp(event) {
        coord2 = getCursorCoordinates(event)
        console.log('Released on' + coord2)
        // get the time of finishing the click on the canvas
        var tapEndAt = (new Date()).getTime()

        var mouseEventsTimeDiff = tapEndAt - tapStartAt

        // if the difference of time between click down and click up is more than 500ms assume it is a swipe, not a tap
        // to allow flick swipes we also check the difference between the gesture coordinates
        // x1, y1 = mousedown coordinates
        // x2, y2 = mouseup coordinates
        // if x2 > x1*1.1 - it is probably a swipe left to right
        // if x2 < x1*0.9 - it is probably a swipe right to left
        // if y2 < y1*0.9 - it is probably a swipe bottom to top
        // if y2 > y1*1.1 - it is probably a swipe top to bottom
        if (mouseEventsTimeDiff > 500 || coord2[0] > coord1[0] * 1.1 || coord2[0] < coord1[0] * 0.9 || coord2[1] < coord1[1] * 0.9 || coord2[1] > coord1[1] * 1.1) {
            swipeCoordinates(authToken, logout, coord1, coord2, streamData, setDialog)
        } else if (mouseEventsTimeDiff < 500) {
            tapCoordinates(authToken, logout, coord1, streamData, setDialog)
        } else {
            touchAndHoldCoordinates(authToken, logout, coord1, streamData, setDialog)
        }
    }

    return (
        <canvas
            id="actions-canvas"
            width={streamData.canvasWidth + 'px'}
            height={streamData.canvasHeight + 'px'}
            onMouseDown={handleMouseDown}
            onMouseUp={handleMouseUp}
            style={{ position: "absolute" }}
        ></canvas>
    )
}

function Stream({ canvasWidth, canvasHeight, streamUrl }) {
    return (
        <img
            id="image-stream"
            width={canvasWidth + 'px'}
            height={canvasHeight + 'px'}
            style={{ display: 'block' }}
            src={streamUrl}
        ></img>
    )
}

// tap using coordinates
function tapCoordinates(authToken, logout, pos, streamData, setDialog) {
    // set initial x and y tap coordinates
    let x = pos[0]
    let y = pos[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        x = (x / streamData.canvasWidth) * streamData.deviceX
        y = (y / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": x,
        "y": y
    })

    let deviceURL = `/device/${streamData.udid}`

    api.post(deviceURL + "/tap", jsonData)
        .then(response => {
            if (response.status === 404) {
                setDialog(true)
                return
            }

            if (response.status === 401) {
                logout()
            }
        })
        .catch(() => {
            setDialog(true)
        })
}

function touchAndHoldCoordinates(authToken, logout, pos, streamData, setDialog) {
    // set initial x and y tap coordinates
    let x = pos[0]
    let y = pos[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        x = (x / streamData.canvasWidth) * streamData.deviceX
        y = (y / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": x,
        "y": y
    })

    let deviceURL = `/device/${streamData.udid}`

    api.post(deviceURL + "/touchAndHold", jsonData)
        .then(response => {
            if (response.status === 404) {
                setDialog(true)
                return
            }

            if (response.status === 401) {
                logout()
            }
        })
        .catch(() => {
            setDialog(true)
        })
}

function swipeCoordinates(authToken, logout, coord1, coord2, streamData, setDialog) {
    var firstCoordX = coord1[0]
    var firstCoordY = coord1[1]
    var secondCoordX = coord2[0]
    var secondCoordY = coord2[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        firstCoordX = (firstCoordX / streamData.canvasWidth) * streamData.deviceX
        firstCoordY = (firstCoordY / streamData.canvasHeight) * streamData.deviceY
        secondCoordX = (secondCoordX / streamData.canvasWidth) * streamData.deviceX
        secondCoordY = (secondCoordY / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": firstCoordX,
        "y": firstCoordY,
        "endX": secondCoordX,
        "endY": secondCoordY
    })

    let deviceURL = `/device/${streamData.udid}`

    api.post(deviceURL + "/swipe", jsonData)
        .then(response => {
            if (response.status === 404) {
                setDialog(true)
                return
            }

            if (response.status === 401) {
                logout()
            }
        })
        .catch(() => {
            setDialog(true)
        })
}

function homeButton(authToken, deviceData, setDialog) {
    let deviceURL = `/device/${deviceData.udid}/home`

    api.post(deviceURL)
        .then(response => {
            if (response.status === 404) {
                setDialog(true)
            }
        })
        .catch(() => {
            setDialog(true)
        })
}

function lockButton(authToken, deviceData, setDialog) {
    let deviceURL = `/device/${deviceData.udid}/lock`

    api.post(deviceURL)
        .then(response => {
            if (response.status === 404) {
                setDialog(true)
            }
        })
        .catch(() => {
            setDialog(true)
        })
}

function unlockButton(authToken, deviceData, setDialog) {
    let deviceURL = `/device/${deviceData.udid}/unlock`

    api.post(deviceURL)
        .then(response => {
            if (response.status === 404) {
                setDialog(true)
            }
        })
        .catch(() => {
            setDialog(true)
        })
}