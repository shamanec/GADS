import { api } from './api'

const axiosInterceptor = (logout) => {
    // intercept the response
    api.interceptors.response.use(
        (response) => {
            return response
        },
        (error) => {
            // Check if the error response status is 401 (Unauthorized)
            if (error.response.status === 401) {
                localStorage.removeItem('authToken');
                localStorage.removeItem('userRole')
                logout()
            }

            // Reject the error to propagate it to the request's catch block
            return Promise.reject(error)
        }
    )
}

export default axiosInterceptor