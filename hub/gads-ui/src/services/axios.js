import axios from 'axios'
import {useContext} from "react";
import {Auth} from "../contexts/Auth";

export function GetAPIClient(ctx) {
    // const [, , logout] = useContext(Auth)
    const api = axios.create({
        baseURL: `http://192.168.1.6:10000`
        // baseURL: ``
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

    api.interceptors.response.use(
        (response) => {
            // Simply return the response if there are no errors
            return response
        },
        (error) => {
            // Check if the error response status is 401 (Unauthorized)
            if (error.response.status === 401) {
                console.log('Unauthorized access - perhaps redirect to login')
                localStorage.removeItem('authToken');
                localStorage.removeItem('userRole')
                // logout()
            }

            // Reject the error to propagate it to the request's catch block
            return Promise.reject(error)
        }
    )

    return api
}