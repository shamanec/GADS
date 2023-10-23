import { useEffect, useRef, useState } from 'react'
import Head from 'next/head'
import { FiArrowLeft } from 'react-icons/fi'
import Router, { useRouter } from 'next/router'
import { createRoot } from 'react-dom/client'

import { Device } from '@/components/Device'
import { HorizontalTab } from '@/components/HorizontalTab'
import { TreeView } from '@/components/TreeView'
import { TreeNodeData } from '@/components/TreeView/TreeNodeItem'
import { SuspendedDevice } from '@/components/SuspendedDevice'

import { useDevice } from '@/hooks/useDevice'
import { useDeviceControl } from '@/hooks/useDeviceControl'

//import { ExternalSocketProvider } from '@/providers/externalSocket'

import styles from '@/styles/Dashboard.module.scss'

export default function Control() {
    const router = useRouter()
    const { query } = router

    //const [toggleContext, setToggleContext] = useState<boolean>(false)
    const { handleGetAvailableDevicesList, handleGetDeviceDetail, deviceControl } = useDevice()
    const {
        tree,
        //setDeviceStream,
        handlePressHomeScreen,
        handleLockDevice,
        handleUnlockDevice,
        getAppSource
    } = useDeviceControl({ device: deviceControl.Device })
    const [option, setOption] = useState<number>(1)
    const canvas = useRef<HTMLCanvasElement | null>(null)
    const [selectedItem, setSelectedItem] = useState<TreeNodeData | null>(null)

    const [socket, setSocket] = useState<WebSocket | null>(null)
    const [isConnected, setIsConnected] = useState<boolean>(false)
    const [message, setMessage] = useState<any>('')

    const items = [
        { id: 1, name: 'Inspect' },
        { id: 2, name: 'Monitoring' },
        { id: 3, name: 'Logs' },
        { id: 4, name: 'Automation' }
    ]

    const handlePushDeviceScreen = () => Router.push('/dashboard/devices')

    const handleGetAppSource = async () => {
        await getAppSource(deviceControl?.Device?.UDID, deviceControl.Device)
    }

    const openDeviceWindow = () => {
        console.log('Appium port: ', deviceControl?.Device.AppiumPort)
        const deviceWindow = window.open(
          '',
          `Device ${deviceControl?.Device.UDID}`,
          `width=${deviceControl.CanvasWidth}, height=${deviceControl.CanvasHeight}`
        );
    
        if (deviceWindow) {
            deviceWindow.document.title = deviceControl?.Device?.Name
          
            const rootElement = deviceWindow.document.createElement('div')
            rootElement.style.width = `${deviceControl.CanvasWidth}px`
            rootElement.style.height = `${deviceControl.CanvasHeight}px`
            deviceWindow.document.body.style.margin = "0"
            deviceWindow.document.body.style.padding = "0"
            deviceWindow.document.body.appendChild(rootElement)

          const root = createRoot(rootElement)
          
          // Renderize o componente SuspendedDevice na nova janela
          root.render(
            <SuspendedDevice
              canvasWidth={Number(deviceControl?.CanvasWidth)}
              canvasHeight={Number(deviceControl?.CanvasHeight)}
              stream={message}
              canvasRef={canvas}
              device={deviceControl?.Device}
            />
          )
        }
    }

    useEffect(() => {
        handleGetAvailableDevicesList()
        if (query.udid) {
            handleGetDeviceDetail(String(query.udid))
            //setToggleContext(true)

            //handle create socket connection here
            const externalSocketInstance = new WebSocket(`ws://${process.env.NEXT_PROVIDER_HOST}/device/${String(query.udid)}/android-stream`)
            console.log(externalSocketInstance)
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
        }
    }, [query.udid])

    useEffect(() => {
        if (socket != null) {
            socket.onmessage = (event: MessageEvent) => {
                const data = event.data
                const messageType = new DataView(data.slice(0, 4)).getInt32(0, false)

                if (messageType === 2) {
                    const imageData = new Uint8Array(data.slice(4))
                    const blob = new Blob([imageData], { type: 'image/jpeg' })
                    const imageURL = URL.createObjectURL(blob)

                    setMessage(imageURL)
                }
            }
        }
    }, [socket])

    useEffect(() => {
        console.log(selectedItem)
    }, [selectedItem])

    return (
        <>
            <Head>
                <title>Device {`${query.udid}`} | GADS</title>
            </Head>
            <main className={styles.contentContainer}>
                <div className={styles.headerSection}>
                    <div className={styles.headerContainer}>
                        <button className={styles.routerButton} onClick={handlePushDeviceScreen}>
                            <FiArrowLeft color="var(--gray-700)" />
                            <span>Go back to devices</span>
                        </button>
                    </div>
                </div>
                <div className={styles.mainSection}>
                    <div className={styles.deviceController}>
                        <div className={styles.device}>
                            <div className={styles.actions}>
                                <button onClick={() => handlePressHomeScreen(deviceControl!.Device!.UDID)}>
                                    <img src='/images/home.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Home button</span>
                                </button>
                                <button>
                                    <img src='/images/back.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Back button</span>
                                </button>
                                <button onClick={() => handleLockDevice(deviceControl!.Device!.UDID)}>
                                    <img src='/images/lock.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Lock device</span>
                                </button>
                                <button onClick={() => handleUnlockDevice(deviceControl!.Device!.UDID)}>
                                    <img src='/images/unlock.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Unlock device</span>
                                </button>
                                <button>
                                    <img src='/images/device.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Rotate device</span>
                                    </button>
                                <button>
                                    <img src='/images/refresh.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Reload device</span>
                                </button>
                                <button>
                                    <img src='/images/camera.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Take screenshot</span>
                                </button>
                                {/* Experimental feat */}
                                <button onClick={openDeviceWindow}>
                                    <img src='/images/suspend.svg' alt='icon' />
                                    <span className={styles.tooltipText}>Open in a new window (Beta)</span>
                                </button>
                            </div>
                            {/*
                                Experimental:
                                Wrap the <Device> component in the socket context (ExternalSocketProvider):
                                "It's showing an error in the browser, it might be the way it's being presented.
                                I added a toggleContext to prevent it from running. Ideally, everything should happen in the context, 
                                so as not to leave the responsibility to the [udid].tsx page.
                            {toggleContext && (
                                <ExternalSocketProvider udid={String(query.udid)}>
                                    <Device
                                        deviceWidth={Number(deviceControl?.CanvasWidth)}
                                        deviceHeight={Number(deviceControl?.CanvasHeight)}
                                        stream={message}
                                        canvasRef={canvas}
                                        device={deviceControl.Device}
                                    />
                                </ExternalSocketProvider>
                            )}
                            */}
                            <Device
                                deviceWidth={Number(deviceControl?.CanvasWidth)}
                                deviceHeight={Number(deviceControl?.CanvasHeight)}
                                stream={message}
                                canvasRef={canvas}
                                device={deviceControl?.Device}
                            />
                            {/* "setDeviceStream is not in use, it should be used when the ExternalSocketProvider is functioning correctly. */}
                        </div>
                        <div className={styles.verticalDivider}></div>
                        <div className={styles.controllers}>
                            <div className={styles.controllersHeader}>
                                <HorizontalTab option={option} setOption={setOption} items={items} />
                            </div>
                            <div className={styles.controllersBody}>
                                {option === 1 && (
                                    <div className={styles.rowContainer}>
                                        <div className={styles.content}>
                                            <div className={styles.contentHeader}>
                                                <img src='/images/hierarchy.svg' alt='hierarchy icon' />
                                                <h2>Elements hierarchy</h2>
                                            </div>
                                            <div className={styles.buttonGroup}>
                                                <button onClick={handleGetAppSource}>
                                                    <img src='/images/source.svg' alt='icon' />
                                                    <span>Get app source</span>
                                                </button>
                                                <div className={styles.divider} />
                                                <button>
                                                    <img src='/images/reload.svg' alt='icon' />
                                                    <span>Reload</span>
                                                </button>
                                            </div>
                                            <div className={styles.tree}>
                                                {tree.title && tree.title.trim() !== '' ? <TreeView data={tree} isSelected={selectedItem === tree} onSelect={setSelectedItem} /> : <p>Click 'Get app source' to load the application's source.</p>}
                                            </div>
                                        </div>
                                        <div className={styles.divider} />
                                        <div className={styles.content}>
                                            <div className={styles.contentHeader}>
                                                <img src='/images/target.svg' alt='target icon' />
                                                <h2>Element info</h2>
                                            </div>
                                            <div className={styles.buttonGroup}>
                                                <button>
                                                    <img src='/images/tap.svg' alt='icon' />
                                                    <span>Tap</span>
                                                </button>
                                                <div className={styles.divider} />
                                                <button>
                                                    <img src='/images/text.svg' alt='icon' />
                                                    <span>Write</span>
                                                </button>
                                                <div className={styles.divider} />
                                                <button>
                                                    <img src='/images/clean.svg' alt='icon' />
                                                    <span>Clean</span>
                                                </button>
                                            </div>
                                            <div className={styles.infoContainer}>
                                                {selectedItem ? (
                                                    <table>
                                                        <thead>
                                                            <tr>
                                                                <th>Attribute</th>
                                                                <th>Value</th>
                                                            </tr>
                                                        </thead>
                                                        <tbody>
                                                            {Object.entries(selectedItem.key).map(([key, value]) => (
                                                                <tr key={key}>
                                                                <td>{key}</td>
                                                                <td>{value}</td>
                                                                </tr>
                                                            ))}
                                                        </tbody>
                                                    </table>
                                                ) : (
                                                    <p>Select an element to inspect it.</p>
                                                )}
                                            </div>
                                        </div>
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </>
    );
}
