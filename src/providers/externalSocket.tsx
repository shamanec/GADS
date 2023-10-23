import { createContext, useContext, useEffect, useState, ReactNode } from "react"

type ExternalSocketContextType = {
  socket: WebSocket | null;
  isConnected: boolean;
}

const ExternalSocketContext = createContext<ExternalSocketContextType>({
  socket: null,
  isConnected: false,
})

export const useExternalSocket = () => {
  return useContext(ExternalSocketContext)
}

type ExternalSocketProviderProps = {
  children: ReactNode;
  udid: string;
}

export const ExternalSocketProvider = ({ children, udid }: ExternalSocketProviderProps) => {
  const [socket, setSocket] = useState<WebSocket | null>(null)
  const [isConnected, setIsConnected] = useState(false)

  useEffect(() => {
    const externalSocketInstance = new WebSocket(`ws://${process.env.NEXT_PROVIDER_HOST}/device/${udid}/android-stream`)
    
    externalSocketInstance.binaryType = 'arraybuffer'

    externalSocketInstance.onopen = () => {
      setIsConnected(true)
    }

    externalSocketInstance.onclose = () => {
      setIsConnected(false)
    }

    setSocket(externalSocketInstance)

    return () => {
      externalSocketInstance.close()
    }
  }, [udid])

  return (
    <ExternalSocketContext.Provider value={{ socket, isConnected }}>
      {children}
    </ExternalSocketContext.Provider>
  )
}
