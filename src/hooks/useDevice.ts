import axios from 'axios'
import { useState } from 'react'

import { Device } from '@/utils/util'

interface DeviceControl {
    Device: Device,
    CanvasWidth: string,
    CanvasHeight: string,
}

export function useDevice() {
    const [devices, setDevices] = useState<Device[]>([])
    const [deviceControl, setDeviceControl] = useState<DeviceControl>({} as DeviceControl)

    const handleGetAvailableDevicesList = async () => {
        try {
          const response = await axios.get('/api/socket/available-devices')
          const devices = response.data
      
          setDevices(devices)
        } catch (error) {
          console.error("Erro ao obter dispositivos disponíveis:", error)
        }
    }

    const handleGetDeviceDetail = async (udid: string) => {
        try {
            const response = await axios.get(`/api/socket/device-control/${udid}`)
            const device = response.data

            setDeviceControl(device)
        } catch (error) {
            console.error("Erro ao obter detalhes do dispositivo:", error)
        }
    }

    return {
        handleGetAvailableDevicesList,
        handleGetDeviceDetail,

        devices,
        deviceControl
    }
}