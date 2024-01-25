import { useState, useEffect, useContext } from 'react'
import { FiPlus } from 'react-icons/fi'

import { Modal } from '../../components/Modal'
import { Badge } from '../../components/Badge'
import { EmptyBlock } from '../../components/EmptyBlock'
import { ProviderTable } from '../../components/ProviderTable'

import { Auth } from '../../contexts/Auth'

import { api } from '../../services/axios'

import styles from '../../styles/Settings.module.scss'
import { validateEmail, validatePassword } from '../../utils/validators'

export default function Admin() {
    const { authToken } = useContext(Auth)

    const [activeView, setActiveView] = useState(1)

    const [userModalOpen, setUserModalOpen] = useState(false)
    const [providerModalOpen, setProviderModalOpen] = useState(false)
    const [message, setMessage] = useState({ visible: false, message: '' })

    const [userForm, setUserForm] = useState({
        email: '',
        password: '',
        username: '',
        role: 'user'
    })
    const [providerForm, setProviderForm] = useState({
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

    const [providers, setProviders] = useState([])

    const [isLoadingCreation, setIsLoadingCreation] = useState(false)

    const handleCloseModal = () => {
        setMessage({ visible: false,  message: '' })

        activeView === 1 ? setUserModalOpen(!userModalOpen) : setProviderModalOpen(!providerModalOpen)
    }

    const handleAddUser = async () => {
        if(userForm.email.trim() === '') {
            return setMessage({ visible: true, message: 'Please, fill the user email'})
        }

        if(userForm.password.trim() === '') {
            return setMessage({ visible: true, message: 'Please, fill the user password'})
        }

        if(userForm.username.trim() === '') {
            return setMessage({ visible: true, message: 'Please, fill the user name'})
        }

        if(!validateEmail(userForm.email)) {
            return setMessage({ visible: true, message: 'Please, enter a valid email' })
        }

        if(!validatePassword(userForm.password)) {
            return setMessage({ visible: true, message: 'Please, enter a valid password' })
        }

        setIsLoadingCreation(true)

        console.log('user fomr >>', userForm)

        await api.post('/admin/user', {
            username: userForm.username,
            password: userForm.password,
            role: userForm.role,
            email: userForm.email
        }, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).then(response => {
            if(response.status === 200) {
                setUserForm({
                    email: '',
                    password: '',
                    role: 'user',
                    username: ''
                })

                setMessage({ visible: false, message: ''})
                setUserModalOpen(!userModalOpen)
            }
        }).catch(error => {
            if (error.response) {
                if (error.response.status === 401) {
                    return
                }

                return
            }
        })

        setIsLoadingCreation(false)
    }

    const handleAddProvider = async () => {
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

        setIsLoadingCreation(true)

        await api.post('/admin/providers/add', {
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
                setProviders(response.data)
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
                setProviderModalOpen(!providerModalOpen)
            }
        }).catch(error => {
            if (error.response) {
                if (error.response.status === 401) {
                    return
                }

                return
            }
        })

        setIsLoadingCreation(false)
    }

    useEffect(() => {
        const fetchData = async () => {
            await api.get('/admin/providers', {
                headers: {
                    'X-Auth-Token': authToken
                }
            }).then(response => {
                setProviders(response.data)
            }).catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        return
                    }
                }
                console.log('Failed getting providers data' + error)
                return
            })
        }

        fetchData()
    }, [])

    return(
        <>
            <Modal
                icon={<FiPlus color='var(--gray-500)' />}
                title='Add user'
                supporting='Add a new user with permissions or not'
                modalOpen={userModalOpen}
            >
                <div className={styles.modalContent}>
                    {message.visible && <Badge type='error' baseText='Erro' contentText={message.message} />}
                    <div className={styles.modalContentForm} style={{ height: 'max-content'}}>
                        <div className={styles.columnGroups}>
                            <label htmlFor='email'>Email</label>
                            <input
                                className={`${message.message?.includes("user email") && styles.error}`}
                                type='text'
                                name='email'
                                id='email'
                                placeholder='Enter with your email address'
                                value={userForm.email}
                                onChange={e => setUserForm({...userForm, email: e.target.value})}
                            />
                        </div>
                        <div className={styles.columnGroups}>
                            <label htmlFor='password'>Password</label>
                            <input
                                className={`${message.message?.includes("user password") && styles.error}`}
                                type='password'
                                name='password'
                                id='password'
                                placeholder='Create a password'
                                value={userForm.password}
                                onChange={e => setUserForm({...userForm, password: e.target.value})}
                            />
                        </div>
                        <div className={styles.columnGroups}>
                            <label htmlFor='username'>Username</label>
                            <input
                                className={`${message.message?.includes("user name") && styles.error}`}
                                type='text'
                                name='username'
                                id='username'
                                placeholder='Create a username'
                                value={userForm.username}
                                onChange={e => setUserForm({...userForm, username: e.target.value})}
                            />
                        </div>
                        <div className={styles.columnGroups}>
                            <label htmlFor='role'>Role</label>
                            <select name='role' id='role' value={userForm.role} onChange={e => setUserForm({...userForm, role: e.target.value})}>
                                <option value='user'>User</option>
                                <option value='admin'>Admin</option>
                            </select>
                        </div>
                    </div>
                    <div className={styles.modalActions}>
                        <button onClick={handleCloseModal}>Close</button>
                        <button onClick={handleAddUser} className={`${isLoadingCreation && styles.loading}`}>
                            <span>Add user</span>
                        </button>
                    </div>
                </div>
            </Modal>
            <Modal
                icon={<FiPlus color='var(--gray-500)' />}
                title='Add provider'
                supporting='Add a new provider for each device block'
                modalOpen={providerModalOpen}
            >
                <div className={styles.modalContent}>
                    {message.visible && <Badge type='error' baseText='Erro' contentText={message.message} />}
                    <div className={styles.modalContentForm}>
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
                        <button onClick={handleAddProvider} className={`${isLoadingCreation && styles.loading}`}>
                            <span>Add provider</span>
                        </button>
                    </div>
                </div>
            </Modal>
            <main className={styles.contentContainer}>
                <div className={styles.mainSection}>
                    <div className={styles.textAndSupportingText}>
                        <h2>Admin settings</h2>
                        <div className={styles.divider}></div>
                    </div>

                    <div className={styles.settingsSection}>
                        <div className={styles.verticalTabs}>
                            <button className={`${activeView === 1 && styles.active}`} onClick={() => setActiveView(1)}>Users</button>
                            <button className={`${activeView === 2 && styles.active}`} onClick={() => setActiveView(2)}>Providers</button>
                        </div>

                        {activeView === 1 && (
                            <div className={styles.settingsForm}>
                                <div className={styles.settingsHeader}>
                                    <div className={styles.textAndSupportingText}>
                                        <h2>User administration</h2>
                                        <span>Manage users and their account permissions here.</span>
                                    </div>
                                    <div className={styles.actions}>
                                        <button className={styles.quickButton} onClick={() => setUserModalOpen(!userModalOpen)}>
                                            <div className={styles.imageBlock}>
                                                <img src='/images/user.svg' alt='user icon' />
                                            </div>
                                            <div className={styles.columnItems}>
                                                <h2>Add a new user</h2>
                                                <span>Create a new common user or administrator.</span>
                                            </div>
                                        </button>
                                    </div>
                                </div>
                                <div className={styles.formContent}>
                                    <EmptyBlock
                                        emptyText='No registered users'
                                        emptySupporting='Register a new common user or administrator and give him roles.'
                                    />
                                </div>
                            </div>
                        )}

                        {activeView === 2 && (
                            <div className={styles.settingsForm}>
                                <div className={styles.settingsHeader}>
                                    <div className={styles.textAndSupportingText}>
                                        <h2>Providers administration</h2>
                                        <span>Create and manage providers for device blocks.</span>
                                    </div>
                                    <div className={styles.actions}>
                                        <button className={styles.quickButton} onClick={() => setProviderModalOpen(!providerModalOpen)}>
                                            <div className={styles.imageBlock}>
                                                <img src='/images/provider.svg' alt='user icon' />
                                            </div>
                                            <div className={styles.columnItems}>
                                                <h2>Add a new provider</h2>
                                                <span>Create a single provider for your devices</span>
                                            </div>
                                        </button>
                                    </div>
                                </div>
                                <div className={styles.formContent}>
                                    {providers.length > 0 ? (
                                        <ProviderTable data={providers} providerForm={providerForm} setProviderForm={setProviderForm} />
                                    ) : (
                                        <EmptyBlock
                                            emptyText='No providers registered'
                                            emptySupporting='Register a new provider for your connected devices'
                                        />
                                    )}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </main>
        </>
    )
}