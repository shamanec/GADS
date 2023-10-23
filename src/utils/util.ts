import axios from 'axios'
import r from 'rethinkdb'

import { newConnection } from './db'

export interface Device {
    AppiumPort: string;
    AppiumSessionID: string;
    Connected: boolean;
    Container: {
        ContainerID: string;
        ContainerName: string;
        ContainerStatus: string;
        ImageName: string;
    };
    ContainerServerPort: string;
    Healthy: boolean;
    Host: string;
    Image: string;
    LastHealthyTimestamp: number;
    Model: string;
    Name: string;
    OS: string;
    OSVersion: string;
    ScreenSize: string;
    StreamPort: string;
    UDID: string;
    WDAPort: string;
    WDASessionID: string;
}

interface AppiumGetSessionsResponse {
    value: { id: string }[];
}
  
interface AppiumCreateSessionResponse {
    value: { sessionId: string };
}

export async function getLatestDBDevices():Promise<Device[]> {
    try {
        const conn = await newConnection()

        let cursor = await r.table('devices').run(conn)
        let devices = await cursor.toArray()

        return devices
    }catch(error) {
        console.log('Could not get devices from DB: ', error)
        return []
    }
    //while(true) {
      

    //    await new Promise((resolve) => setTimeout(resolve, 1000))
    //}
}

export async function checkWDASection(wdaURL: string): Promise<string> {
    try{
        const response = await axios.get(`http://${wdaURL}/status`)
        const responseJson = response.data

        if(!responseJson.sessionId) {
            const sessionId = await createWDASession(wdaURL)
            if(sessionId) {
                return sessionId
            }
        }

        return responseJson.sessionId
    } catch(error) {
        return Promise.reject(error)
    }
}

export async function createWDASession(wdaURL: string): Promise<string> {
    const requestObjects = {
        capabilities: {
            firstMatch: [
                {
                    arguments: [],
					environment: {},
					eventloopIdleDelaySec: 0,
					shouldWaitForQuiescence: true,
					shouldUseTestManagerForVisibilityDetection: false,
					maxTypingFrequency: 60,
					shouldUseSingletonTestManager: true,
					shouldTerminateApp: true,
					forceAppLaunch: true,
					useNativeCachingStrategy: true,
					forceSimulatorSoftwareKeyboardPresence: false
                }
            ],
            alwaysMatch: {}
        },
    }

    try{
        const response = await axios.post(`http://${wdaURL}/session`, requestObjects)
        const responseJson = response.data

        if(!responseJson.sessionId) {
            return Promise.reject(new Error('Could not get `sessionId` while creating a new WebDriverAgent session'))
        }

        return responseJson.sessionId
    }catch(error) {
        return Promise.reject(error)
    }
}

export async function checkAppiumSession(appiumURL: string): Promise<string> {
    try{
        const response = await axios.get(`http://${appiumURL}/sessions`)
        const responseJson: AppiumGetSessionsResponse = response.data

        if (responseJson.value.length === 0) {
            const sessionId = await createAppiumSession(appiumURL)
            return sessionId
        }

        return responseJson.value[0].id
    } catch(error) {
        return Promise.reject(error)
    }
}

export async function createAppiumSession(appiumURL: string): Promise<string> {
    const requestObject = {
        capabilities: {
            alwaysMatch: {
                'appium:automationName': 'UiAutomator2',
                platformName: 'Android',
                'appium: ensureWebviewsHavePages': true,
                'appium:nativeWebScreenshot': true,
                'appium:newCommandTimeout': 0,
                'appium:connectHardwareKeyboard': true,
            },
            firstMatch: [{}],
        },
        desiredCapabilities: {
            'appium:automationName': 'UiAutomator2',
            platformName: 'Android',
            'appium:ensureWebviewsHavePages': true,
            'appium:nativeWebScreenshot': true,
            'appium:newCommandTimeout': 0,
            'appium:connectHardwareKeyboard': true,
        }
    }

    try{
        const response = await axios.post(`http://${appiumURL}/session`, requestObject)
        const responseJson: AppiumCreateSessionResponse = response.data

        return responseJson.value.sessionId
    }catch(error) {
        return Promise.reject(error)
    }
}

export function getDeviceByUDID(udid: string, latestDevices: Device[]): Device | undefined {
    return latestDevices.find((device) => device.UDID === udid)
}