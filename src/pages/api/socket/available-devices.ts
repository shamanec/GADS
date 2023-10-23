import { NextApiRequest } from 'next'
import r from 'rethinkdb'

import { NextApiResponseServerIo } from '../../../../types'

import { newConnection } from '@/utils/db'

export default async function handler(
  req: NextApiRequest,
  res: NextApiResponseServerIo
) {
  if(req.method !== "GET") {
    return res.status(405).json({ error: "Method not allowed" })
  }

  try {
    res?.socket?.server?.io?.emit("Connected")

    const conn = await newConnection()

    let cursor = await r.table('devices').run(conn)
    let devices = await cursor.toArray()

    res?.socket?.server?.io?.emit(JSON.stringify(devices))
    return res.status(200).json(devices)
  } catch (error) {
    console.log("[ERRO]", error)
    return res.status(500).json({ message: "Internal server error" })
  }
}

/*import type { NextApiRequest, NextApiResponse } from 'next'
import WebSocket from 'ws'

import { getLatestDBDevices } from '@/utils/util'

const connections: WebSocket[] = []

export default async (req: NextApiRequest, res: NextApiResponse) => {
  if(req.method === 'GET') {
    const wss = new WebSocket.Server({ noServer: true })

    wss.on('connection', (ws) => {
      connections.push(ws)

      const sendAvailableDevices = setInterval(() => {
        if(ws.readyState == ws.OPEN) {
          getLatestDBDevices().then((availableDevices) => {
            ws.send(JSON.stringify(availableDevices))
          })
        }
      }, 2000)

      ws.on('message', (message) => {
        //Handle message
      })

      ws.on('close', () => {
        //Remove connection
        const index = connections.indexOf(ws)
        if(index !== -1) {
          connections.splice(index, 1)
        }

        clearInterval(sendAvailableDevices)
      })
    })

    wss.handleUpgrade(req, req.socket, Buffer.alloc(0), (ws) => {
      wss.emit('connection', ws, res)
    })
  }else {
    res.status(405).end()
  }
}*/