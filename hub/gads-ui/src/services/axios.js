import axios from 'axios'

export function GetAPIClient() {
    const api = axios.create({
        // baseURL: `http://192.168.1.41:10000`
        baseURL: ``
    })

    // api.interceptors.request.use(
    //     async (config) => {
    //         // Verificar se config é um objeto válido
    //         if (!config || typeof config !== 'object') {
    //             console.error('Config inválido no interceptor:', config)
    //             return Promise.reject(new Error('Config inválido no interceptor'))
    //         }

    //         // Ensure headers object exists
    //         config.headers = config.headers || {}
            
    //         const accessToken = localStorage.getItem('accessToken') || ''

    //         if (accessToken && typeof accessToken === 'string' && accessToken.trim() !== '') {
    //             config.headers['Authorization'] = `Bearer ${accessToken}`
    //         }

    //         return config
    //     },
    //     (error) => {
    //         console.error('Erro no interceptor de requisição:', error)
    //         return Promise.reject(error)
    //     }
    // )

    
    // Add a request interceptor
    api.interceptors.request.use(function (config) {
        // Do something before request is sent
        const accessToken = localStorage.getItem('accessToken') || ''
        if (accessToken) {
            config.headers = {
                ...config.headers,
                authorization: `Bearer ${accessToken}`
            }
        }
        return config;
    }, function (error) {
        // Do something with request error
        return Promise.reject(error);
    });

    return api
}