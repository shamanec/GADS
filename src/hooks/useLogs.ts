import { useEffect, useState } from 'react'

interface LogsProps {

}

export function useLogs() {
    const [logs, setLogs] = useState<LogsProps[]>([])

    useEffect(() => {
        function getInitialLogs() {
            fetch("/project-logs", {
                method: 'GET',
            }).then((data) => {
                return data.text()
            }).then((response) => {
                console.log('Response >>', response)
                //createLogsTable(response) TODO: setar o response para a variável logs
            }).catch(function(error) {
                console.log("Provider not available", "Could not get logs: " + error, "error")
            })
        }

        getInitialLogs()
    }, [])

    function getLogs(url: string) {
        fetch(url, {
            method: 'GET',
        }).then(data => {
            return data.text()
        }).then((response) => {
            console.log('Response >>', response)
            //createLogsTable(response) TODO: setar o response para a variável logs
        }).catch(function(error) {
            console.log("Provider not available", "Could not get logs: " + error, "error")
        })
    }

    return { logs, getLogs }
}