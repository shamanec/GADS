import { useEffect, useCallback, useState } from 'react'
import axios from 'axios'

import { useDeviceControl } from '@/hooks/useDeviceControl'

import { Device } from '@/utils/util'

import styles from './styles.module.scss'

interface DeviceProps {
  deviceWidth: number;
  deviceHeight: number;
  stream: string;
  canvasRef: React.RefObject<HTMLCanvasElement>;
  device: Device
}

export function Device({ deviceWidth, deviceHeight, stream, canvasRef, device }: DeviceProps) {
  const { getCursorCoordinates, applyCursorRippleEffect } = useDeviceControl({ device: device })

  const [url, setUrl] = useState<string | null>(null)

  const renderImage = useCallback(async () => {
    const canvas = canvasRef.current
    const context = canvas?.getContext('2d')

    if (!canvas || !context) {
      return
    }

    const url = stream

    if (!url) {
      return
    }

    setUrl(url)

    const image = new Image()
    image.src = url

    image.onload = () => {
      canvas.width = deviceWidth
      canvas.height = deviceHeight
      context.drawImage(image, 0, 0, deviceWidth, deviceHeight)
    }
  }, [stream, deviceWidth, deviceHeight])

  useEffect(() => {
    renderImage()

    const canvas = canvasRef.current

    if(canvas) {
      canvas.addEventListener('mousedown', handleCanvasMouseDown)
      canvas.addEventListener('mouseup', handleCanvasMouseUp)

      return () => {
        canvas.removeEventListener('mousedown', handleCanvasMouseDown)
        canvas.removeEventListener('mouseup', handleCanvasMouseUp)
      }
    }
  }, [renderImage, stream, url])

  let tapStartAt = 0
  let coord1: number[] = []
  let coord2: number[] = []

  const handleCanvasMouseDown = (e: MouseEvent) => {
    tapStartAt = (new Date()).getTime()
    coord1 = getCursorCoordinates(canvasRef.current, e)
    applyCursorRippleEffect(e)
  }

  const handleCanvasMouseUp = (event: MouseEvent) => {
    coord2 = getCursorCoordinates(canvasRef.current, event)
    const tapEndAt = (new Date()).getTime()

    const mouseEventsTimeDiff = tapEndAt - tapStartAt

    // if the difference of time between click down and click up is more than 600ms assume it is a swipe, not a tap
    // to allow flick swipes we also check the difference between the gesture coordinates
    // x1, y1 = mousedown coordinates
    // x2, y2 = mouseup coordinates
    if (mouseEventsTimeDiff > 500 || coord2[0] > coord1[0] * 1.1 || coord2[0] < coord1[0] * 0.9 || coord2[1] < coord1[1] * 0.9 || coord2[1] > coord1[1] * 1.1) {
      performSwipe(coord1, coord2)
    } else {
      // else perform a simple tap at coordinates
      tapCoordinates(coord1)
    }
  }

  const tapCoordinates = async (pos: number[]) => {
    // get device screen size dimensions
    let dimensions = device.ScreenSize.split('x')
    let height = parseInt(dimensions[1], 10)
    let width = parseInt(dimensions[0], 10)

    // set initial x and y tap coordinates
    let x = pos[0]
    let y = pos[1]

    // if the stream height 
    if (deviceHeight != height) {
      x = (pos[0] / deviceWidth) * width
      y = (pos[1] / deviceHeight) * height
    }

    let jsonData = JSON.stringify({
      "x": x,
      "y": y
    })

    return await axios.post(`http://${process.env.NEXT_PROVIDER_HOST}/device/${device.UDID}/tap`, jsonData)
      .then(() => {})
      .catch(error => console.log('Tap failed. Error: ', error))
  }

  const performSwipe = async (coord1: number[], coord2: number[]) => {
    var firstCoordX = coord1[0]
    var firstCoordY = coord1[1]
    var secondCoordX = coord2[0]
    var secondCoordY = coord2[1]

    console.log('x1: ', coord1[0])
    console.log('y1: ', coord1[1])
    console.log('x2: ', coord2[0])
    console.log('y2: ', coord2[1])

    // get device screen size dimensions
    let dimensions = device.ScreenSize.split('x')
    let height = parseInt(dimensions[1], 10)
    let width = parseInt(dimensions[0], 10)

    // if the stream height 
    if (deviceHeight != height) {
        firstCoordX = (firstCoordX / deviceWidth) * width
        firstCoordY = (firstCoordY / deviceHeight) * height
        secondCoordX = (secondCoordX / deviceWidth) * width
        secondCoordY = (secondCoordY / deviceHeight) * height
    }

    let jsonData = JSON.stringify({
        "x": firstCoordX,
        "y": firstCoordY,
        "endX": secondCoordX,
        "endY": secondCoordY
    })

    if(firstCoordX === null && firstCoordY === null) {
      return
    }

    return await axios.post(`http://${process.env.NEXT_PROVIDER_HOST}/device/${device.UDID}/swipe`, jsonData)
      .then(() => {})
      .catch(error => console.log('Swipe failed. Error: ', error))
  }

  return (
    <>
      <canvas
        ref={canvasRef}
        id='actions-canvas'
        className={styles.deviceContainer}
        style={{ width: `${deviceWidth}px`, height: `${deviceHeight}px` }}
      />
    </>
    
  );
}
