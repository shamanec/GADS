import { useState } from 'react'

export function useDevice() {
    const [devices, setDevices] = useState([])

    const checkServerHealth = async () => {
        try{
            const response = await api.get(`/health`)

            if(response.status === 200) {
                return {
                    success: true,
                    message: 'Server healthy',
                    response: response.data
                }
            }else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response.data
                }
            }
        }catch(error) {
            if(error.response) {
                if(error.response.status === 403) {
                    return {
                        success: false,
                        message: 'Incomplete request',
                        response: error.response
                    }
                }
            }

            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    return {
        devices,
        setDevices,

        checkServerHealth
    }
}