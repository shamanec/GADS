import { NextApiRequest, NextApiResponse } from 'next'
import axios from 'axios'

import { Device } from '@/components/DeviceTable'

import { getLatestDBDevices } from '@/utils/util'

export default async (req: NextApiRequest, res: NextApiResponse) => {
    const { method, query } = req
    const { udid } = query

    switch(method) {
        case 'GET':
            try{
                const latestDevices = await getLatestDBDevices()
                const device = getDBDevice(udid, latestDevices)

                if(!device.UDID) {
                    res.status(500).json({ error: 'Device not found' })
                    return
                }
    
                const url = `http://${device.Host}:10001/device/${device.UDID}/health`
    
                const response = await axios.get(url)

                if(response.status !== 200) {
                    const body = response.data
                    res.status(500).json({ error: `Device not healthy: ${body}` })
                    return
                }

                const { canvasWidth, canvasHeight } = calculateCanvasDimensions(device.ScreenSize)

                const pageData = {
                    Device: device,
                    CanvasWidth: canvasWidth,
                    CanvasHeight: canvasHeight,
                }

                return res.json(pageData)
            }catch(error) {
                res.status(500).json({ error: error })
            }

            break
        default:
            res.status(405).end()
    }
}

function getDBDevice(udid: string | string[] | undefined, latestDevices: Device[]): Device {
    for(const dbDevice of latestDevices) {
        if(dbDevice.UDID === udid) {
            return dbDevice
        }
    }

    return {} as Device
}

function calculateCanvasDimensions(size: string) {
    const dimensions = size.split('x')
    const widthString = dimensions[0]
    const heightString = dimensions[1]

    const width = parseInt(widthString)
    const height = parseInt(heightString)

    const screenRatio = width / height

    const canvasHeight = '850'
    const canvasWidth = (850 * screenRatio).toString()

    return { canvasWidth, canvasHeight }
}