import axios from 'axios'

export function GetAPIClient() {
    const api = axios.create({
        // baseURL: `http://192.168.1.6:10000`
        baseURL: ``
    })

    api.interceptors.request.use(
        async (config) => {
            const storedToken = localStorage.getItem('authToken')

            if(storedToken) {
                config.headers['X-Auth-Token'] = `${storedToken}`
            }

            return config
        }
    )

    return api
}