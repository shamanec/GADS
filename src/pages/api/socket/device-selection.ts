import { NextApiRequest, NextApiResponse } from 'next'
import WebSocket, { WebSocketServer } from 'ws'

import { getLatestDBDevices, Device } from '@/utils/util'

const wss = new WebSocketServer({ noServer: true })

const clients = new Set<WebSocket>()

wss.on('connection', (ws) => {
    clients.add(ws)

    ws.on('close', () => {
        clients.delete(ws)
    })

    ws.on('message', (message) => {
        //Handle WebSocket messages if needed
    })

    //Send the initial device selection HTML
    sendDeviceSelectionHTML(ws)
})

function sendDeviceSelectionHTML(ws: WebSocket) {
    getLatestDBDevices()
        .then((latestDevices) => {
            const html = generateDeviceSelectionHTML(latestDevices)
            ws.send(html)

            //TODO: send update to clients when the device list changes calling sendDeviceSelectionHTML(ws) again
        }).catch((error) => {
            console.error('Error getting latest devices: ', error)
        })
}

function generateDeviceSelectionHTML(latestDevices: Device[]) {
    //Generate the device selection HTML with the latest data
    const html = `
        <ul>
            ${latestDevices.map((device) => `<li>${device.Name}</li>`).join('')}
        </ul>
    `

    return html
}

export default (req: NextApiRequest, res: NextApiResponse) => {
    if(req.method === 'GET') {
        wss.handleUpgrade(req, req.socket, Buffer.alloc(0), (ws) => {
            wss.emit('connection', ws, req)
        })
    }else {
        res.status(405).end()
    }
}