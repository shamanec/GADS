import styles from './styles.module.scss'

export interface LogData {
    date: string,
    type: string,
    event: string,
    message: string
}

interface LogTableProps {
    data: LogData[]
}

export function LogTable({ data }: LogTableProps) {
    return(
        <div className={styles.tableContainer}>
            <table className={styles.table}>
                <thead>
                    <tr>
                        <th>Date/Time</th>
                        <th>Level</th>
                        <th>Event</th>
                        <th>Message</th>
                    </tr>
                </thead>
                <tbody>
                    {data.map((log, index) => {
                        return(
                            <tr key={index}>
                                <td>{log.date}</td>
                                <td><span className={`${styles.levelType} ${log.type === 'Error' ? styles.error : styles.warning}`}>{log.type}</span></td>
                                <td>{log.event}</td>
                                <td>{log.message}</td>
                            </tr>
                        )
                    })}
                </tbody>
            </table>
        </div>
    )
}