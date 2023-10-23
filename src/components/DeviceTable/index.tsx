import { Dispatch, SetStateAction, useEffect, useState } from 'react'
import { FiMoreVertical } from 'react-icons/fi'

import useTable from '@/hooks/useTable'

import styles from './styles.module.scss'

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
  

interface DeviceTableProps {
    cardTitle: string,
    data: Device[],
    rowsPerPage: number
}

interface TableFooterProps {
    range: any[],
    setPage: Dispatch<SetStateAction<number>>,
    page: number,
    slice: any[]
}

export function DeviceTable({ cardTitle, data, rowsPerPage }: DeviceTableProps) {
    const [page, setPage] = useState(1)
    const { slice, range } = useTable(data, page, rowsPerPage)
    
    return(
        <div className={styles.tableContainer}>
            <div className={styles.cardHeader}>
                <span>{ cardTitle }</span>
                <button className={styles.actions}>
                    <FiMoreVertical color='var(--gray-200)' />
                </button>
            </div>
            <table className={styles.table}>
                <thead>
                    <tr>
                        <th>Device</th>
                        <th>Model</th>
                        <th>Screen size</th>
                        <th>Status</th>
                        <th></th>
                    </tr>
                </thead>
                <tbody>
                    {
                        data.map((device, index) => {
                            return(
                                <tr key={index}>
                                    <td>
                                        <div className={styles.tableCellContainer}>
                                            <div className={styles.deviceImage}>
                                                <img src={`${device?.OS === 'android' ? '/images/android.svg' : '/images/ios.svg'}`} alt='device os image' />
                                            </div>
                                            <div className={styles.deviceInfo}>
                                                <h2>{ device.Name }</h2>
                                                <span>{ device?.OS === 'android' ? `Android ${device?.OSVersion}` : `iOS ${device?.OSVersion}` }</span>
                                            </div>
                                        </div>
                                    </td>
                                    <td>{ device?.Model }</td>
                                    <td>{ device?.ScreenSize }</td>
                                    <td>
                                        <span className={`${styles.deviceStatus} ${device.Connected? styles.available : styles.unavailable}`}>
                                            { device.Connected ? 'Available' : 'Unavailable' }
                                        </span>
                                    </td>
                                    <td>
                                        <div className={styles.actionButtons}>
                                            <button>Detail</button>
                                            {device.Connected && <a href={`/dashboard/devices/control/${device.UDID}`}>Use device</a>}
                                        </div>
                                    </td>
                                </tr>
                            )
                        })
                    }
                </tbody>
            </table>
            <TableFooter range={range} slice={slice} setPage={setPage} page={page} />
        </div>
    )
}

const TableFooter = ({ range, setPage, page, slice }: TableFooterProps) => {
    useEffect(() => {
        if(slice.length < 1 && page !== 1) {
            setPage(page - 1)
        }
    }, [slice, page, setPage])

    return(
        <div className={styles.tableFooter}>
            {range.map((el, index) => (
                    <button
                        key={index}
                        className={`${styles.button} ${
                            page === el ? styles.activeButton : styles.inactiveButton
                        }`}
                        onClick={() => setPage(el)}
                    >
                        {el}
                    </button>
            ))}

            <span>Page {page} of {range.length}</span>
        </div>
    )
}