import { Auth } from "../../contexts/Auth"
import { useContext, useEffect, useState } from "react"
import './StreamCanvas.css'
import { Box, Button, Divider, Grid, Stack, Tooltip } from "@mui/material"
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
    const [isPortrait, setIsPortrait] = useState(true)
    const handleOrientationButtonClick = (isPortrait) => {
        setIsPortrait(isPortrait);
    }

    let deviceX = parseInt(deviceData.screen_width, 10)
    let deviceY = parseInt(deviceData.screen_height, 10)
    let screen_ratio = deviceX / deviceY
    let landscapeScreenRatio = deviceY / deviceX

    const streamData = {
        udid: deviceData.udid,
        deviceX: deviceX,
        deviceY: deviceY,
        screen_ratio: screen_ratio,
        canvasHeight: canvasSize.height,
        canvasWidth: canvasSize.width,
        isPortrait: isPortrait,
        device_os: deviceData.os,
        uses_custom_wda: deviceData.uses_custom_wda
    }

    let streamUrl = ""
    if (deviceData.os === 'ios') {
        streamUrl = `http://192.168.1.41:10000/device/${deviceData.udid}/ios-stream-mjpeg`
        // streamUrl = `/device/${deviceData.udid}/ios-stream-mjpeg`
    } else {
        streamUrl = `http://192.168.1.41:10000/device/${deviceData.udid}/android-stream-mjpeg`
        // streamUrl = `/device/${deviceData.udid}/android-stream-mjpeg`
    }

    useEffect(() => {
        const updateCanvasSize = () => {
            let canvasWidth, canvasHeight
            if (isPortrait) {
                canvasHeight = window.innerHeight * 0.7
                canvasWidth = canvasHeight * screen_ratio
            } else {
                canvasWidth = window.innerWidth * 0.4
                canvasHeight = canvasWidth / landscapeScreenRatio
            }

            setCanvasSize({
                width: canvasWidth,
                height: canvasHeight
            })
        }

        const imgElement = document.getElementById('image-stream');

        // Temporarily remove the stream source
        imgElement.src = '';

        updateCanvasSize()

        // Reapply the stream URL after the resize is complete
        imgElement.src = streamUrl;

        // Set resize listener
        window.addEventListener('resize', updateCanvasSize);

        return () => {
            window.stop()
            window.removeEventListener('resize', updateCanvasSize);
        }
    }, [isPortrait]);

    return (
        <Grid
            spacing={1}
            direction="row"
            display="flex"
            justifyContent='center'
            alignItems='flex-start'
        >

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
                <Grid
                    height='30px'
                    display='flex'
                    justifyContent='center'
                >
                </Grid>
            </div >

            <Grid
                direction="column"
                width="150px"
                marginLeft="10px"
                spacing={1}
                container
            >
                <Grid item>
                    <Tooltip
                        title="This does not change the orientation of the device itself, just updates the UI if the device orientation is already changed"
                        arrow
                        placement='top'
                    >
                        <Button
                            variant={"contained"}
                            color={"secondary"}
                            onClick={() => handleOrientationButtonClick(true)}
                            disabled={isPortrait}
                            sx={{ width: '100%' }}
                        >
                            Portrait
                        </Button>
                    </Tooltip>
                </Grid>
                <Grid item>
                    <Tooltip
                        title="This does not change the orientation of the device itself, just updates the UI if the device orientation is already changed"
                        arrow
                        placement='top'
                    >
                        <Button
                            variant={"contained"}
                            color={"secondary"}
                            onClick={() => handleOrientationButtonClick(false)}
                            disabled={!isPortrait}
                            sx={{ width: '100%' }}
                        >
                            Landscape
                        </Button>
                    </Tooltip>
                </Grid>
                <Grid item>
                    <Divider></Divider>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => homeButton(authToken, deviceData, setDialog)}
                        className='canvas-buttons'
                        startIcon={<HomeIcon />}
                        variant='contained'
                        style={{
                            fontWeight: "bold",
                            color: "#9ba984",
                            backgroundColor: "#2f3b26",
                            width: '100%'
                        }}
                    >Home</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => lockButton(authToken, deviceData, setDialog)}
                        className='canvas-buttons'
                        startIcon={<LockIcon />}
                        variant='contained'
                        style={{
                            fontWeight: "bold",
                            color: "#9ba984",
                            backgroundColor: "#2f3b26",
                            width: '100%'
                        }}
                    >Lock</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => unlockButton(authToken, deviceData, setDialog)}
                        className='canvas-buttons'
                        startIcon={<LockOpenIcon />}
                        variant='contained'
                        style={{
                            fontWeight: "bold",
                            color: "#9ba984",
                            backgroundColor: "#2f3b26",
                            width: '100%'
                        }}
                    >Unlock</Button>
                </Grid>
            </Grid>
        </Grid >
    )
}

function Canvas({ authToken, logout, streamData, setDialog }) {
    var tapStartAt = 0
    var coord1
    var coord2

    function getCursorCoordinates(event) {
        const rect = event.currentTarget.getBoundingClientRect()
        const x = event.clientX - rect.left
        const y = event.clientY - rect.top
        return [x, y];
    }

    function handleMouseDown(event) {
        tapStartAt = (new Date()).getTime()
        coord1 = getCursorCoordinates(event)
    }

    function handleMouseUp(event) {
        coord2 = getCursorCoordinates(event)
        // get the time of finishing the click on the canvas
        var tapEndAt = (new Date()).getTime()

        console.log('Tap on ' + coord1[0] + 'x' + coord1[1] + ', released on ' + coord2[0] + 'x' + coord2[1])

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
    console.log("tapping")
    // set initial x and y tap coordinates
    let x = pos[0]
    let y = pos[1]

    let finalX = (x / streamData.canvasWidth) * streamData.deviceX
    let finalY = (y / streamData.canvasHeight) * streamData.deviceY
    // If its portrait we keep the x and y as is
    if (!streamData.isPortrait) {
        // If its landscape
        // And its Android we still keep the x and y as is because for Android Appium does the coordinates are actually corresponding to the view
        // If we are in portrait X is X and Y is Y and when we are in landscape its the same
        if (streamData.device_os === 'android') {
            finalX = (x / streamData.canvasHeight) * streamData.deviceX
            finalY = (y / streamData.canvasWidth) * streamData.deviceY
        }
        if (streamData.device_os === 'ios') {
            if (streamData.uses_custom_wda) {
                finalX = streamData.deviceX - ((y / streamData.canvasHeight) * streamData.deviceX)
                finalY = (x / streamData.canvasWidth) * streamData.deviceY
            } else {
                // On normal WDA in landscape X corresponds to the canvas width but it is actually the device's height
                finalX = (x / streamData.canvasWidth) * streamData.deviceY
                // Y corresponds to the canvas height but it is actually the device's width
                finalY = (y / streamData.canvasHeight) * streamData.deviceX
            }
        }
    }

    let jsonData = JSON.stringify({
        "x": finalX,
        "y": finalY
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

    // Set up the portrait tap coordinates
    // We divide the current coordinate by the canvas size to get the ratio
    // Then we multiply by the actual device width or height to get the actual coordinates on the device that we should send
    let firstXFinal = (firstCoordX / streamData.canvasWidth) * streamData.deviceX
    let firstYFinal = (firstCoordY / streamData.canvasHeight) * streamData.deviceY
    let secondXFinal = (secondCoordX / streamData.canvasWidth) * streamData.deviceX
    let secondYFinal = (secondCoordY / streamData.canvasHeight) * streamData.deviceY

    // If in landscape we need to do different recalculations
    if (!streamData.isPortrait) {
        // If the device is android we just reverse the calculations
        // For X we divide the coordinate by the canvas height to get the correct ratio
        // And for Y we divide the coordinate by the canvas width to get the correct ratio
        // Multiplication is the same as for portrait because Appium for Android follows some actual logic
        if (streamData.device_os === 'android') {
            firstXFinal = (firstCoordX / streamData.canvasHeight) * streamData.deviceX
            firstYFinal = (firstCoordY / streamData.canvasWidth) * streamData.deviceY
            secondXFinal = (secondCoordX / streamData.canvasHeight) * streamData.deviceX
            secondYFinal = (secondCoordY / streamData.canvasWidth) * streamData.deviceY
        } else {
            // For iOS its complete sh*t
            if (streamData.uses_custom_wda) {
                // NB: All calculations below are when the device is on its right side landscape(your left)
                // For custom WDA the 0 X coordinate is at the bottom of the canvas
                // And the 0 Y coordinate is at the left end of the canvas
                // This means that they kinda follow the portrait logic but inverted based on the side it is in landscape
                // Imagine a device that is X:Y=100:200
                // If you swipe vertically from bottom to the middle you are essentially swiping coordinates on the device X:0 and X: 50
                // And if you swipe horizontally from left to middle you are essentially swiping coordinates on the device Y: 0 and Y: 100
                // But on the canvas those are Y coordinates
                // And this is where it gets funky
                // For X we get the ratio from the canvas Y coordinate and the canvas height
                // And we multiply it by the device width because that is what it corresponds to
                // Then we subtract the value from the actual deviceX because the device width starts from the bottom of the canvas height
                firstXFinal = streamData.deviceX - ((firstCoordY / streamData.canvasHeight) * streamData.deviceX)
                // For Y we get the ratio from the canvas X coordinate and the canvas width
                // And we multiply it by the device height because that is what it corresponds to
                firstYFinal = (firstCoordX / streamData.canvasWidth) * streamData.deviceY
                // Same goes for the other two coordinates when the mouse is released
                secondXFinal = streamData.deviceX - ((secondCoordY / streamData.canvasHeight) * streamData.deviceX)
                secondYFinal = (secondCoordX / streamData.canvasWidth) * streamData.deviceY
            } else {
                // On normal WDA the X coordinates correspond to the device height
                // and the Y coordinates correspond to the device width
                // So we calculate ratio based on canvas dimensions and coordinates
                // But multiply by device height for X swipe coordinates and by device width for Y swipe coordinates
                firstXFinal = (firstCoordX / streamData.canvasWidth) * streamData.deviceY
                firstYFinal = (firstCoordY / streamData.canvasHeight) * streamData.deviceX
                secondXFinal = (secondCoordX / streamData.canvasWidth) * streamData.deviceY
                secondYFinal = (secondCoordY / streamData.canvasHeight) * streamData.deviceX
            }
        }
    }
    console.log('Swipe is ' + firstXFinal + ':' + firstYFinal + '-' + secondXFinal + ':' + secondYFinal)

    let jsonData = JSON.stringify({
        "x": firstXFinal,
        "y": firstYFinal,
        "endX": secondXFinal,
        "endY": secondYFinal
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