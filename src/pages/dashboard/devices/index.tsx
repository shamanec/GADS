import { ReactNode, useEffect, useState } from 'react'
import Head from 'next/head'
import { FiSearch } from 'react-icons/fi'
import { BsFilter } from 'react-icons/bs'
import axios from 'axios'

import { HorizontalTab } from '@/components/HorizontalTab'
import { Device, DeviceTable } from '@/components/DeviceTable'

import { useSocket } from '@/providers/socket'

import styles from '@/styles/Dashboard.module.scss'

interface DeviceTableOption {
  id: number,
  icon?: ReactNode,
  option: string,
  label: string
}

export default function Devices() {
  
  const [option, setOption] = useState(1)
  const [devices, setDevices] = useState<Device[]>([])

  const deviceTableOptions: DeviceTableOption[] = [
    {id: 1, icon: null, option: 'all', label: 'All devices'},
    {id: 2, icon: null, option: 'android', label: 'Android devices'},
    {id: 3, icon: null, option: 'ios', label: 'iOS devices'}
  ]

  const handleOptionChange = (id: number) => setOption(id)

  const filteredData: Device[] = option != 1 ? devices?.filter(d => d.OS.toLowerCase().includes(deviceTableOptions[option - 1].option.toLowerCase())) : devices

  const { isConnected } = useSocket()

  const handleGetAvailableDevicesList = async () => {
    try {
      const response = await axios.get('/api/socket/available-devices')
      const devices = response.data
  
      setDevices(devices)
    } catch (error) {
      console.error("Error retrieving available devices: ", error)
    }
  }

  useEffect(() => {
    if(isConnected) {
      handleGetAvailableDevicesList()
    }

  }, [isConnected])

  return (
    <>
      <Head>
        <title>Devices - GADS</title>
      </Head>

      <main className={styles.contentContainer}>
        <div className={styles.headerSection}>
          <div className={styles.headerContainer}>
            <div className={styles.content}>
              <h2>Available devices</h2>
              <span>Visualize and control the device of your choice.</span>
            </div>
            <div className={styles.deviceTableOptions}>
              {deviceTableOptions.map(o => {
                  return(
                      <button key={o.id} className={`${option === o.id && styles.active}`} onClick={() => setOption(o.id)}>
                          {o.icon}
                          <span>{o.label}</span>
                      </button>
                  )
              })}
            </div>
          </div>
        </div>
        <div className={styles.mainSection}>
          <div className={styles.searchAndFilters}>
            <div className={styles.searchbox}>
              <FiSearch color='var(--gray-500)' />
              <input type='text' placeholder='Search for device' />
            </div>
            <button className={styles.filterButton}>
              <BsFilter color='var(--gray-700)' />
              Filters
            </button>
          </div>
          <div className={styles.devicesTable}>
            <DeviceTable cardTitle={deviceTableOptions[option - 1].label} data={filteredData} rowsPerPage={5} />
          </div>
        </div>
      </main>
    </>
  )
}