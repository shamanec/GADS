import { useState, useEffect, useContext } from 'react'

import { Modal } from '../Modal'
import { Badge } from '../Badge'

import { api } from '../../services/axios'

import { Auth } from '../../contexts/Auth'

import styles from './styles.module.scss'
  
export function ProviderTable({ data, providerForm, setProviderForm }) {
    const { authToken, logout } = useContext(Auth)

    let infoSocket = null
    let onlineSocket = null
    const [selectedProvider, setSelectedProvider] = useState({})

    const [infoModalOpen, setInfoModalOpen] = useState(false)

    const [message, setMessage] = useState({ visible: false, message: '' })
    const [isLoadingUpdate, setIsLoadingUpdate] = useState(false)

    const [isLoadingProviderInfo, setIsLoadingProviderInfo] = useState(false)
    const [isOnline, setIsOnline] = useState(false)
    const [devices, setDevices] = useState([])


    const handleCloseModal = () => {
        setMessage({ visible: false, message: '' })
        setProviderForm({
            os: 'linux',
            hostAddress: '',
            nickname: '',
            port: 0,
            android: false,
            ios: false,
            useSeleniumGrid: false,
            seleniumGrid: '',
            supervisionPassword: '',
            wdaBundleId: '',
            wdaRepoPath: ''
        })

        setInfoModalOpen(!infoModalOpen)
    }

    const handleSelectProvider = (provider) => {
        setInfoModalOpen(!infoModalOpen)
        setSelectedProvider(provider)

        if(infoSocket === null) {
            if (infoSocket) {
                infoSocket.close()
            }
            infoSocket = new WebSocket(`ws://${process.env.REACT_APP_PROVIDER_HOST}/admin/provider/${provider.nickname}/info-ws`);
    
            infoSocket.onerror = (error) => {
                setIsOnline(false)
                setIsLoadingProviderInfo(false)
            };
    
            infoSocket.onclose = () => {
                setIsOnline(false)
                setIsLoadingProviderInfo(false)
            }
    
            infoSocket.onmessage = (message) => {
    
                let providerJSON = JSON.parse(message.data)
                setProviderForm({
                    os: providerJSON.os,
                    hostAddress: providerJSON.host_address,
                    nickname: providerJSON.nickname,
                    port: providerJSON.port,
                    android: providerJSON.provide_android,
                    ios: providerJSON.provide_ios,
                    useSeleniumGrid: providerJSON.use_selenium_grid,
                    seleniumGrid: providerJSON.selenium_grid,
                    supervisionPassword: providerJSON.supervision_password,
                    wdaBundleId: providerJSON.wda_bundle_id,
                    wdaRepoPath: providerJSON.wda_repo_path,
                })
                setDevices(providerJSON.provided_devices)
    
                let unixTimestamp = new Date().getTime();
                let diff = unixTimestamp - providerJSON.last_updated
                if (diff > 4000) {
                    setIsOnline(false)
                } else {
                    setIsOnline(true)
                }
    
                if (isLoadingProviderInfo) {
                    setIsLoadingProviderInfo(false)
                }
    
                if (infoSocket) {
                    infoSocket.close()
                }
            }
    
            return () => {
                if (infoSocket) {
                    infoSocket.close()
                    infoSocket = null
                }
            }
        }
    }

    const handleClickReset = (deviceInfo) => {
        //TODO: Create logic for this
        api.post(`/device/${deviceInfo.udid}/reset`, null, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).catch((error) => {
            if (error.response) {
                if (error.response.status === 401) {
                    logout()
                }
            }
        })
    }

    const handleUpdateProvider = async () => {
        if(providerForm.nickname.trim() === '') {
            return setMessage({ visible: true, message: 'Please, fill the provider name'})
        }

        if(providerForm.hostAddress.trim() === '') {
            return setMessage({ visible: true, message: 'Please, fill the host address'})
        }

        if(providerForm.port === 0) {
            return setMessage({ visible: true, message: 'Please, fill the host port'})
        }

        if(providerForm.ios) {
            if(providerForm.wdaBundleId.trim() === '') {
                return setMessage({ visible: true, message: 'Please, fill the bundle identifier'})
            }
        }

        if(providerForm.useSeleniumGrid) {
            if(providerForm.seleniumGrid.trim() === '') {
                return setMessage({ visible: true, message: 'Please, fill the selenium grid'})
            }
        }

        setIsLoadingUpdate(true)

        await api.post('/admin/providers/update', {
            os: providerForm.os,
            host_address: providerForm.hostAddress,
            nickname: providerForm.nickname,
            port: Number(providerForm.port),
            provide_android: providerForm.android,
            provide_ios: providerForm.ios,
            wda_bundle_id: providerForm.wdaBundleId,
            wda_repo_path: providerForm.wdaRepoPath,
            supervision_password: providerForm.supervisionPassword,
            use_selenium_grid: providerForm.useSeleniumGrid,
            selenium_grid: providerForm.seleniumGrid
        }, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).then(response => {
            if(response.status === 200) {
                setProviderForm({
                    os: 'linux',
                    hostAddress: '',
                    nickname: '',
                    port: 0,
                    android: false,
                    ios: false,
                    useSeleniumGrid: false,
                    seleniumGrid: '',
                    supervisionPassword: '',
                    wdaBundleId: '',
                    wdaRepoPath: ''
                })

                setMessage({ visible: false, message: ''})
                setInfoModalOpen(!infoModalOpen)
            }
        }).catch(error => {
            if (error.response) {
                if (error.response.status === 401) {
                    return
                }

                return
            }
        })

        setIsLoadingUpdate(false)
    }

    useEffect(() => {
        if (onlineSocket) {
            onlineSocket.close()
        }
        onlineSocket = new WebSocket(`ws://${process.env.REACT_APP_PROVIDER_HOST}/admin/provider/${selectedProvider.nickname}/info-ws`);

        onlineSocket.onerror = (error) => {
            setIsOnline(false)
        };

        onlineSocket.onclose = () => {
            setIsOnline(false)
        }

        onlineSocket.onmessage = (message) => {
            let providerJSON = JSON.parse(message.data)

            let unixTimestamp = new Date().getTime();
            let diff = unixTimestamp - providerJSON.last_updated
            if (diff > 4000) {
                setIsOnline(false)
            } else {
                setIsOnline(true)
            }
        }

        return () => {
            if (onlineSocket) {
                onlineSocket.close()
            }
        }
    }, [isOnline])

    return(
        <>
            <Modal
                icon={selectedProvider.os === 'linux' ? <img style={{ width: '32px'}} src='./images/linux.svg' alt='linux icon'/> : selectedProvider.os === 'windows' ? <img style={{ width: '32px'}} src='./images/windows.svg' alt='windows icon' /> : <img style={{ width: '32px'}} src='./images/darwin.svg' alt='darwin icon' />}
                title={`${selectedProvider.nickname} info`}
                block={
                    <div className={styles.providerInfo}>
                        <span style={{ color: `${isOnline ? 'var(--green-700)' : 'var(--red-550)'}`}}>&#9679;</span>
                        <span>{isOnline ? 'Active now' : 'Inactive'}</span>
                    </div>
                }
                supporting='View provider info and update your settings'
                modalOpen={infoModalOpen}
            >
                <div className={styles.modalContent}>
                    {message.visible && <Badge type='error' baseText='Erro' contentText={message.message} />}
                    {isLoadingProviderInfo ? (
                        <div className={styles.boxLoading} style={{ height: '480px'}}>
                            <div className={styles.loading} />
                        </div>
                    ) : (
                        <>
                            <div className={styles.modalContentForm}>
                            <div className={styles.devicesInfo}>
                                <h3>Devices list</h3>
                                {!isOnline || devices !== null ? (
                                    <span>No device data or provider offline</span>
                                ): <span>TODO (I need to do)</span>}
                            </div>
                                <div className={styles.rowGroups}>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='os'>Operational system</label>
                                        <select name='os' id='os' value={providerForm.os} onChange={e => setProviderForm({...providerForm, os: e.target.value})}>
                                            <option value='linux'>Linux</option>
                                            <option value='windows'>Windows</option>
                                            <option value='dawin'>MacOS</option>
                                        </select>
                                    </div>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='nickname'>Nickname*</label>
                                        <input
                                            className={`${message.message?.includes("provider name") && styles.error}`}
                                            type='text'
                                            name='nickname'
                                            id='nickname'
                                            placeholder='Type a nickname'
                                            value={providerForm.nickname}
                                            onChange={e => setProviderForm({...providerForm, nickname: e.target.value})}
                                        />
                                        <span className={styles.textHint}>Unique nickname for the provider</span>
                                    </div>
                                </div>
                                <div className={styles.rowGroups}>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='host'>Host address*</label>
                                        <input
                                            className={`${message.message?.includes("host address") && styles.error}`}
                                            type='text'
                                            name='host'
                                            id='host'
                                            placeholder='Ex: 192.168.1.10'
                                            value={providerForm.hostAddress}
                                            onChange={e => setProviderForm({...providerForm, hostAddress: e.target.value})}
                                        />
                                        <span className={styles.textHint}>
                                            Local IP address of the provider host without scheme, e.g. 192.168.1.10
                                        </span>
                                    </div>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='port'>Port*</label>
                                        <input
                                            className={`${message.message?.includes("host port") && styles.error}`}
                                            type='text'
                                            name='port'
                                            id='port'
                                            placeholder='Ex: 10001'
                                            value={providerForm.port}
                                            onChange={e => setProviderForm({...providerForm, port: Number(e.target.value)})}
                                        />
                                        <span className={styles.textHint}>
                                            The port on which you want the provider instance to run
                                        </span>
                                    </div>
                                </div>
                                <div className={styles.rowGroups}>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='android'>Provide Android?</label>
                                        <select
                                            name='android'
                                            id='android'
                                            value={providerForm.android ? 'yes' : 'no'}
                                            onChange={e => setProviderForm({...providerForm, android: e.target.value === 'yes' ? true : false})}
                                        >
                                            <option value='no'>No</option>
                                            <option value='yes'>Yes</option>
                                        </select>
                                    </div>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='ios'>Provide iOS?</label>
                                        <select
                                            name='ios'
                                            id='ios'
                                            value={providerForm.ios ? 'yes' : 'no'}
                                            onChange={e => setProviderForm({...providerForm, ios: e.target.value === 'yes' ? true : false})}
                                        >
                                            <option value='no'>No</option>
                                            <option value='yes'>Yes</option>
                                        </select>
                                    </div>
                                </div>
                                {providerForm.ios && (
                                    <div className={styles.rowGroups}>
                                        <div className={styles.columnGroups}>
                                            <label htmlFor='bundleid'>WebDriverAgent bundle ID*</label>
                                            <input
                                                className={`${message.message?.includes("bundle identifier") && styles.error}`}
                                                type='text'
                                                name='bundleid'
                                                id='bundleid'
                                                placeholder='e.g. com.facebook.WebDriverAgentRunner.xctrunner'
                                                value={providerForm.wdaBundleId}
                                                onChange={e => setProviderForm({...providerForm, wdaBundleId: e.target.value})}
                                            />
                                            <span className={styles.textHint}>
                                                Bundle ID of the prebuilt WebDriverAgent.ipa, used by 'go-ios' to start it
                                            </span>
                                        </div>
                                        <div className={styles.columnGroups}>
                                            <label htmlFor='path'>WebDriverAgent repo path</label>
                                            <input
                                                type='text'
                                                name='path'
                                                id='path'
                                                placeholder='Ex: /Users/shamanec/WebDriverAgent-5.8.3'
                                                value={providerForm.wdaRepoPath}
                                                onChange={e => setProviderForm({...providerForm, wdaRepoPath: e.target.value})}
                                            />
                                            <span className={styles.textHint}>
                                                Path on the host to WebDriveragent repo to build from, e.g. /Users/shamanec/WebDriverAgent:5.8.3
                                            </span>
                                        </div>
                                    </div>
                                )}
                                
                                <div className={styles.rowGroups}>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='password'>Supervision password</label>
                                        <input
                                            type='text'
                                            name='password'
                                            id='password'
                                            placeholder='Optional'
                                            value={providerForm.supervisionPassword}
                                            onChange={e => setProviderForm({...providerForm, supervisionPassword: e.target.value})}
                                        />
                                        <span className={styles.textHint}>
                                            Password for the supervision profile for iOS devices (leave empty if devices not supervised)
                                        </span>
                                    </div>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='selenium'>Use Selenium Grid?</label>
                                        <select
                                            name='selenium'
                                            id='selenium'
                                            value={providerForm.useSeleniumGrid ? 'yes' : 'no'}
                                            onChange={e => setProviderForm({...providerForm, useSeleniumGrid: e.target.value === 'yes' ? true : false})}
                                        >
                                            <option value='no'>No</option>
                                            <option value='yes'>Yes</option>
                                        </select>
                                    </div>
                                </div>
                                <div className={styles.rowGroups}>
                                    <div className={styles.columnGroups}>
                                        <label htmlFor='seleniumgrid'>Selenium Grid address</label>
                                        <input
                                            className={`${message.message?.includes("selenium grid") && styles.error}`}
                                            type='text'
                                            name='seleniumgrid'
                                            id='seleniumgrid'
                                            placeholder='Optional'
                                            value={providerForm.seleniumGrid}
                                            onChange={e => setProviderForm({...providerForm, seleniumGrid: e.target.value})}
                                        />
                                        <span className={styles.textHint}>
                                            Address of the Selenium Grid instance, e.g. https://192.168.1.28:4444
                                        </span>
                                    </div>
                                </div>
                            </div>
                            <div className={styles.modalActions}>
                                <button onClick={handleCloseModal}>Close</button>
                                <button onClick={handleUpdateProvider} className={`${isLoadingUpdate && styles.loading}`}>
                                    <span>Update provider</span>
                                </button>
                            </div>
                        </>
                    )}
                </div>
            </Modal>
            <div className={styles.tableContainer}>
                {data.map(provider => (
                    <button key={provider.nickname} className={styles.providerBox} onClick={() => handleSelectProvider(provider)}>
                        <div className={styles.imageBlock}>
                            {provider.os === 'linux' ? <img src='/images/linux.svg' alt='linux icon' /> : provider.os === 'darwin' ? <img src='/images/darwin.svg' alt='darwin icon' /> : <img src='/images/windows.svg' alt='windows icon' />}
                        </div>
                        <div className={styles.columnItems}>
                            <h2>{provider.nickname}</h2>
                            <div className={styles.rowItems}>
                                <div className={styles.itemGroup}>
                                    <div className={styles.iconContainer}>
                                        <img src='./images/provider.svg' alt='provider icon' />
                                    </div>
                                    <span>{provider.host_address}</span>
                                </div>
                                {provider.provide_android && <img src='./images/android.svg' alt='android icon' />}
                                {provider.provide_ios && <img src='./images/ios.svg' alt='ios icon' />}
                            </div>
                        </div>
                    </button>   
                ))}
            </div>
        </>
    )
}