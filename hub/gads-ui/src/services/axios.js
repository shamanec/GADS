import axios from 'axios'

export function GetAPIClient() {
    const api = axios.create({
        // baseURL: `http://192.168.1.41:10000`
        baseURL: ``
    })

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