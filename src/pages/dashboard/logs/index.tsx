import Head from 'next/head'

import { LogData, LogTable } from '@/components/LogTable'

import styles from '@/styles/Dashboard.module.scss'

export default function Logs() {

    const mockedLogData: LogData[] = [
        {
            date: '01/05/2023 às 09:16',
            type: 'Error',
            event: 'send_devices_over_ws',
            message: 'This is a mocked data'
        },
        {
            date: '01/05/2023 às 09:16',
            type: 'Error',
            event: 'send_devices_over_ws',
            message: 'This is a mocked data'
        },
    ]

    return(
        <>
            <Head>
                <title>Logs - GADS</title>
            </Head>

            <main className={styles.contentContainer}>
                <div className={styles.headerSection}>
                    <div className={styles.headerContainer}>
                        <div className={styles.content}>
                            <h2>Log tracking</h2>
                            <span>View and analyze container logs.</span>
                        </div>
                    </div>
                </div>
                <div className={styles.mainSection}>
                    <div className={styles.logsTable}>
                        <LogTable data={mockedLogData} />
                    </div>
                </div>
            </main>
        </>
    )
}