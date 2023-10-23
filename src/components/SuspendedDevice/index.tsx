import { useEffect } from 'react'
import axios from 'axios'

import { useDeviceControl } from '@/hooks/useDeviceControl'

import { Device } from '@/utils/util'

interface SuspendedDeviceProps {
  canvasWidth: number;
  canvasHeight: number;
  stream: string;
  canvasRef: React.RefObject<HTMLCanvasElement>;
  device: Device;
}

export function SuspendedDevice({
  canvasWidth,
  canvasHeight,
  stream,
  canvasRef,
  device,
}: SuspendedDeviceProps) {
  const { getCursorCoordinates, applyCursorRippleEffect } = useDeviceControl({
    device: device,
  })

  useEffect(() => {
    const canvas = canvasRef.current

    if (canvas) {
      canvas.addEventListener('mousedown', handleCanvasMouseDown)
      canvas.addEventListener('mouseup', handleCanvasMouseUp)

      return () => {
        canvas.removeEventListener('mousedown', handleCanvasMouseDown)
        canvas.removeEventListener('mouseup', handleCanvasMouseUp)
      }
    }
  }, [stream, canvasWidth, canvasHeight, canvasRef])

  let tapStartAt = 0
  let coord1: number[] = []
  let coord2: number[] = []

  const handleCanvasMouseDown = (e: MouseEvent) => {
    tapStartAt = new Date().getTime()
    coord1 = getCursorCoordinates(canvasRef.current, e)
    applyCursorRippleEffect(e)
  }

  const handleCanvasMouseUp = (event: MouseEvent) => {
    coord2 = getCursorCoordinates(canvasRef.current, event)
    const tapEndAt = new Date().getTime()

    const mouseEventsTimeDiff = tapEndAt - tapStartAt

    if (
      mouseEventsTimeDiff > 500 ||
      coord2[0] > coord1[0] * 1.1 ||
      coord2[0] < coord1[0] * 0.9 ||
      coord2[1] < coord1[1] * 0.9 ||
      coord2[1] > coord1[1] * 1.1
    ) {
      performSwipe(coord1, coord2)
    } else {
      tapCoordinates(coord1)
    }
  }

  const tapCoordinates = async (pos: number[]) => {
    // Observe a validade das coordenadas, pois coordenadas inválidas podem ser NaN ou undefined.
    if (isNaN(pos[0]) || isNaN(pos[1])) {
      return
    }

    let dimensions = device.ScreenSize.split('x')
    let height = parseInt(dimensions[1], 10)
    let width = parseInt(dimensions[0], 10)

    let x = pos[0]
    let y = pos[1]

    if (canvasHeight !== height) {
      x = (pos[0] / canvasWidth) * width
      y = (pos[1] / canvasHeight) * height
    }

    let jsonData = JSON.stringify({
      x: x,
      y: y,
    })

    try {
      await axios.post(`http://${process.env.NEXT_PROVIDER_HOST}/device/${device.UDID}/tap`, jsonData)
    } catch (error) {
      console.log('Tap failed. Error: ', error)
    }
  }

  const performSwipe = async (coord1: number[], coord2: number[]) => {
    var firstCoordX = coord1[0]
    var firstCoordY = coord1[1]
    var secondCoordX = coord2[0]
    var secondCoordY = coord2[1]

    if (isNaN(firstCoordX) || isNaN(firstCoordY)) {
        return
    }
  
    // get device screen size dimensions
    let dimensions = device.ScreenSize.split('x')
    let height = parseInt(dimensions[1], 10)
    let width = parseInt(dimensions[0], 10)

    // if the stream height 
    if (canvasHeight != height) {
        firstCoordX = (firstCoordX / canvasWidth) * width
        firstCoordY = (firstCoordY / canvasHeight) * height
        secondCoordX = (secondCoordX / canvasWidth) * width
        secondCoordY = (secondCoordY / canvasHeight) * height
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
        style={{ width: `${canvasWidth}px`, height: `${canvasHeight}px` }}
      />
    </>
  )
}
