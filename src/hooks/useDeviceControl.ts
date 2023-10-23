import { RefObject, useState } from 'react'
import axios from 'axios'

import { useExternalSocket } from '@/providers/externalSocket'

import { Device } from '@/utils/util'

interface ElementBounds {
    start: { x: number; y: number };
    end: { x: number; y: number };
}
  
interface Json {
    type: string;
    name: string;
    label: string;
    value: string;
    enabled: string;
    visible: string;
    x: number;
    y: number;
    width: number;
    height: number;
    bounds?: ElementBounds;
}

interface AndroidNode {
    nodeName: string;
    nodeType: number;
    getAttribute(attr: string): string | null;
    childNodes: AndroidNode[];
}

interface IOSNode {
    nodeName: string;
    nodeType: number;
    getAttribute(attr: string): string | null;
    childNodes: AndroidNode[];
}

interface AndroidNodeJSON {
    index: string | null;
    package: string | null;
    class: string | null;
    text: string | null;
    checkable: string | null;
    checked: string | null;
    clickable: string | null;
    enabled: string | null;
    focusable: string | null;
    focused: string | null;
    longclickable: string | null;
    password: string | null;
    scrollable: string | null;
    selected: string | null;
    bounds: string | null;
    resourceid: string | null;
    displayed: string | null;
}

interface IOSNodeJSON {
    elemType: string | null;
    elemX: string | null;
    elemY: string | null;
    elemWidth: string | null;
    elemHeight: string | null;
    elemName: string | null;
    elemLabel: string | null;
    elemVisible: string | null;
    elemEnabled: string | null;
    elemValue: string | null;
}

interface IosElement {
    type: string;
    name: string;
    label: string;
    value: string;
    enabled: string;
    visible: string;
    x: string;
    y: string;
    width: string;
    height: string;
}
  
interface AndroidElement {
    index: string;
    resourceid: string;
    package: string;
    class: string;
    text: string;
    checkable: string;
    checked: string;
    clickable: string;
    enabled: string;
    focusable: string;
    focused: string;
    longclickable: string;
    password: string;
    scrollable: string;
    selected: string;
    bounds: string;
    displayed: string;
}

interface TreeNode {
    key: {
      [key: string]: string;
    };
    icon: boolean;
    lazy: boolean;
    active: boolean;
    title: string;
    children?: TreeNode[];
}
  
interface ElementInfo {
    ios: IosElement;
    android: AndroidElement;
}

type DeviceOs = 'ios' | 'android'

interface DeviceControlProps {
    device: Device
}

export function useDeviceControl({ device }: DeviceControlProps) {
    const { socket } = useExternalSocket()

    const [tree, setTree] = useState<TreeNode>({} as TreeNode)

    function setDeviceStream(): Promise<string | null> {
        console.log('socket: ', socket)
        return new Promise((resolve) => {
            if (!device || !socket) {
                resolve(null)
                return
            }

            socket.onmessage = (event: MessageEvent) => {
                const data = event.data
                const messageType = new DataView(data.slice(0, 4)).getInt32(0, false)
        
                if (messageType === 2) {
                  const imageData = new Uint8Array(data.slice(4))
                  const blob = new Blob([imageData], { type: 'image/jpeg' })
                  const imageURL = URL.createObjectURL(blob)
        
                  console.log(imageURL)
                  resolve(imageURL)
                }
            }

            /*
            if (device?.OS === "ios") {
                externalSocket.binaryType = "arraybuffer";
          
                externalSocket.onmessage = (event: any) => {
                    const data = event.data;
                    const messageType = new DataView(data.slice(0, 4)).getInt32(0, false);
          
                    if (messageType === 2) {
                        const imageData = new Uint8Array(data.slice(4));
                        const blob = new Blob([imageData], { type: "image/jpeg" });
                        const imageURL = URL.createObjectURL(blob);
            
                        resolve(imageURL);
                    }
                };
            }

            if (device.OS === "android") {
                externalSocket.binaryType = "arraybuffer";
          
                externalSocket.onmessage = (event: any) => {
                    const data = event.data;
                    const messageType = new DataView(data.slice(0, 4)).getInt32(0, false);
          
                    if (messageType === 2) {
                        const imageData = new Uint8Array(data.slice(4));
                        const blob = new Blob([imageData], { type: "image/jpeg" });
                        const imageURL = URL.createObjectURL(blob);
            
                        resolve(imageURL);
                    }
                };
            }
            */
        })
    }

    //Device actions
    const handlePressHomeScreen = async (udid: string) => {
        return await axios.post(`http://${process.env.NEXT_PROVIDER_HOST}/device/${udid}/home`)
            .then(() => {})
            .catch((error) => {
                console.log('Could not access device homescreen endpoint. Error: ', error)
            })
    }

    const handleLockDevice = async (udid: string) => {
        return await axios.post(`http://${process.env.NEXT_PROVIDER_HOST}/device/${udid}/lock`)
            .then(() => {})
            .catch((error) => {
                console.log('Could not access device lock device endpoint. Error: ', error)
            })
    }

    const handleUnlockDevice = async (udid: string) => {
        return await axios.post(`http://${process.env.NEXT_PROVIDER_HOST}/device/${udid}/unlock`)
            .then(() => {})
            .catch((error) => {
                console.log('Could not access device unlock device endpoint. Error: ', error)
            })
    }

    function getCursorCoordinates(canvas: HTMLCanvasElement | null, event: MouseEvent) {
        if (!canvas) {
          return [0, 0]
        }
    
        const rect = canvas.getBoundingClientRect()
        const x = event.clientX - rect.left
        const y = event.clientY - rect.top
        return [x, y]
    }

    function applyCursorRippleEffect(e: MouseEvent) {
        const ripple = document.createElement("div")
    
        ripple.className = "ripple"
        document.body.appendChild(ripple)
    
        ripple.style.left = `${e.clientX}px`
        ripple.style.top = `${e.clientY}px`
    
        ripple.style.animation = "ripple-effect .2s linear"
        ripple.onanimationend = () => document.body.removeChild(ripple)
    }
    
    function wipeCanvas(canvas: HTMLCanvasElement) {
        if(canvas) {
            const context = canvas.getContext('2d')
            if(context) {
                context.clearRect(0, 0, canvas.width, canvas.height)
            }
        }
    }

    const getAppSource = async (udid: string, device: Device): Promise<void> => {
        try {
            const url = `http://${process.env.NEXT_PROVIDER_HOST}/device/${udid}/appiumSource`
            const response = await axios.get(url)
            const data = response.data
            const parser = new DOMParser()
            const xmlDoc = parser.parseFromString(data.value, 'text/xml')
      
            const jsonTree = generateJSONForTreeFromXML(xmlDoc.documentElement as unknown as AndroidNode | IOSNode, device)
            console.log('app source: ', JSON.parse(jsonTree) as unknown as TreeNode)
            // Faça o que for necessário com a árvore JSON aqui, por exemplo, atualize o estado com ela
            setTree(JSON.parse(jsonTree) as unknown as TreeNode)
        } catch (error) {
          console.error('Could not get app source. Error: ', error)
        }
    }

    function generateJSONForTreeFromXML(node: AndroidNode | IOSNode, device: Device): string {
        if (node.nodeType !== 1) return ''
      
        let valueJson: AndroidNodeJSON | IOSNodeJSON = {} as AndroidNodeJSON | IOSNodeJSON;
      
        if (device?.OS === 'android') {
            // Obtenha todos os dados do documento XML retornado pelo Appium para Android
            // Certifique-se de que as chaves correspondam ao seu objeto JSON
            const index = node.getAttribute('index') || ''
            const packageValue = node.getAttribute('package') || ''
            const elementClass = node.getAttribute('class') || ''
            const text = node.getAttribute('text') || ''
            const checkable = node.getAttribute('checkable') || ''
            const checked = node.getAttribute('checked') || ''
            const clickable = node.getAttribute('clickable') || ''
            const enabled = node.getAttribute('enabled') || ''
            const focusable = node.getAttribute('focusable') || ''
            const focused = node.getAttribute('focused') || ''
            const longClickable = node.getAttribute('long-clickable') || ''
            const password = node.getAttribute('password') || ''
            const scrollable = node.getAttribute('scrollable') || ''
            const selected = node.getAttribute('selected') || ''
            const bounds = node.getAttribute('bounds') || ''
            const displayed = node.getAttribute('displayed') || ''
            const resourceId = node.getAttribute('resource-id') || ''
      
            valueJson = {
                index,
                package: packageValue,
                class: elementClass,
                text,
                checkable,
                checked,
                clickable,
                enabled,
                focusable,
                focused,
                longclickable: longClickable,
                password,
                scrollable,
                selected,
                bounds,
                resourceid: resourceId,
                displayed,
            }
        } else if (device?.OS === 'ios') {
            // Obtenha todos os dados do documento XML retornado pelo Appium para iOS
            // Certifique-se de que as chaves correspondam ao seu objeto JSON
            const elemType = node.getAttribute('type') || ''
            const elemX = node.getAttribute('x') || ''
            const elemY = node.getAttribute('y') || ''
            const elemWidth = node.getAttribute('width') || ''
            const elemHeight = node.getAttribute('height') || ''
            const elemName = node.getAttribute('name') || ''
            const elemLabel = node.getAttribute('label') || ''
            const elemVisible = node.getAttribute('visible') || ''
            const elemEnabled = node.getAttribute('enabled') || ''
            const elemValue = node.getAttribute('value') || ''
      
            valueJson = {
                elemType,
                elemX,
                elemY,
                elemWidth,
                elemHeight,
                elemName,
                elemLabel,
                elemVisible,
                elemEnabled,
                elemValue,
            }
        }
      
        const jsonForChildNodes = Array.from(node.childNodes)
            .map((childNode) => generateJSONForTreeFromXML(childNode, device))
            .join('')
      
        const mainJson = `{"key":${JSON.stringify(valueJson)}, "icon": false, "lazy": false, "active": false,"title":"${node.nodeName}"${
            jsonForChildNodes ? `, "children":[${jsonForChildNodes}]` : ''
        }}`
      
        return mainJson.replace(/}{/g, '},{')
    }

    /*
    function drawRectangleForSelectedElement(json: Json, screenSize: string, deviceOS: string): void {
        //clear the canvas before drawing a new object
        wipeCanvas()
        //TODO: Do reference to React element
        const canvas = document.getElementById('actions-canvas') as HTMLCanvasElement
        if(canvas) {
            const context = canvas.getContext('2d')
            if(context) {
                context.clearRect(0, 0, canvas.width, canvas.height)
                context.fillStyle = 'rgba(33, 109, 255, 0.2)'

                const streamHeight = canvas.height;
                const streamWidth = canvas.width;

                // get device screen size dimensions
                const dimensions = screenSize.split('x');
                const deviceHeight = parseInt(dimensions[1], 10);
                const deviceWidth = parseInt(dimensions[0], 10);

                let elementX: number;
                let elementY: number;
                let elementWidth: number;
                let elementHeight: number;

                if (deviceOS === 'ios') {
                    // get the selected element coordinates and width/height from the json
                    elementX = json.x;
                    elementY = json.y;
                    elementWidth = json.width;
                    elementHeight = json.height;
                } else {
                    if (!json.bounds) {
                    throw new Error('Bounds not found in JSON');
                    }

                    const boundsArray = [
                    json.bounds.start.x,
                    json.bounds.start.y,
                    json.bounds.end.x,
                    json.bounds.end.y,
                    ];

                    // Set the element actual coordinates based on the final arrays
                    // Element starting X is the first value of the first array
                    elementX = boundsArray[0];
                    // Element starting Y is the second value of the first array
                    elementY = boundsArray[1];
                    // Element width is equal to the ending X coordinate from the second array minus the starting X coordinate from the first array
                    elementWidth = boundsArray[2] - boundsArray[0];
                    // Element height is equal to the ending Y coordinate from the second array minus the starting Y coordinate from the first array
                    elementHeight = boundsArray[3] - boundsArray[1];
                }

                // If the stream height is different than the device screen height
                // Recalculate the element coordinates and width/height
                if (streamHeight !== deviceHeight) {
                    elementX = (elementX / deviceWidth) * streamWidth;
                    elementY = (elementY / deviceHeight) * streamHeight;
                    elementWidth = (elementWidth / deviceWidth) * streamWidth;
                    elementHeight = (elementHeight / deviceHeight) * streamHeight;
                }

                // draw the rectangle on the canvas using the recalculated element coordinate values
                context.fillRect(elementX, elementY, elementWidth, elementHeight);

                console.log(elementX);
                console.log(elementY);
                console.log(elementWidth);
                console.log(elementHeight);
            }
        }
    }


    function showElementInfo(json: ElementInfo[DeviceOs]) {
        if (device?.OS === "ios") {
          const iosJson = json as IosElement;
          $("#element-type").text(iosJson.type);
          $("#element-name").text(iosJson.name);
          $("#element-label").text(iosJson.label);
          $("#element-value").text(iosJson.value);
          $("#element-enabled").text(iosJson.enabled);
          $("#element-visible").text(iosJson.visible);
          $("#element-x").text(iosJson.x);
          $("#element-y").text(iosJson.y);
          $("#element-width").text(iosJson.width);
          $("#element-height").text(iosJson.height);
        } else {
          const androidJson = json as AndroidElement;
          $("#element-index").text(androidJson.index);
          $("#element-resourceid").text(androidJson.resourceid);
          $("#element-package").text(androidJson.package);
          $("#element-class").text(androidJson.class);
          $("#element-text").text(androidJson.text);
          $("#element-checkable").text(androidJson.checkable);
          $("#element-checked").text(androidJson.checked);
          $("#element-clickable").text(androidJson.clickable);
          $("#element-enabled").text(androidJson.enabled);
          $("#element-focusable").text(androidJson.focusable);
          $("#element-focused").text(androidJson.focused);
          $("#element-long-clickable").text(androidJson.longclickable);
          $("#element-password").text(androidJson.password);
          $("#element-scrollable").text(androidJson.scrollable);
          $("#element-selected").text(androidJson.selected);
          $("#element-bounds").text(androidJson.bounds);
          $("#element-displayed").text(androidJson.displayed);
        }
    }
    */

    return {
        device,
        tree,

        setDeviceStream,
        handlePressHomeScreen,
        handleLockDevice,
        handleUnlockDevice,
        getCursorCoordinates,
        applyCursorRippleEffect,
        wipeCanvas,
        getAppSource
        /*
        wipeCanvas,
        drawRectangleForSelectedElement,
        getAppSource,
        generateJSONForTreeFromXML,
        showElementInfo
        */
    }
}