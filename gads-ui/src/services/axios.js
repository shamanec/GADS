import axios from 'axios'

export function getAPIClient(ctx) {
    const api = axios.create({
        baseURL: `http://${process.env.REACT_APP_PROVIDER_HOST}`
    })

    api.interceptors.request.use(
        async (config) => {
            const storedToken = localStorage.getItem('authToken')
            
            if(storedToken) {
                config.headers['X-Auth-Token'] = `${storedToken}`
            }

            return config
        },
        async (error) => {
            return Promise.reject(error)
        }
    )

    api.interceptors.response.use(
        (res) => {
            return res
        },
        async (err) => {
            const originalConfig = err.config
            if(err.response) {
                if (err.response.status === 401 && !originalConfig._retry) {
                    originalConfig._retry = true
      
                    const storedToken = localStorage.getItem('authToken')
                    if(storedToken) {
                        try {
                            api.defaults.headers.common['X-Auth-Token'] = storedToken
                
                            return api(originalConfig)
                        } catch (_error) {
                            return Promise.reject(_error)
                        }
                    }
                
                    if (err.response.status === 403 && err.response.data) {
                        return Promise.reject(err.response.data);
                    }
                }
            }

            return Promise.reject(err)
        }
    )

    return api
}