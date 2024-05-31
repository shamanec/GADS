import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import Gads from './Gads';
import reportWebVitals from './reportWebVitals';
import { BrowserRouter as Router } from 'react-router-dom';
import { AuthProvider } from './contexts/Auth';
import {api} from "./services/api";

function checkServerHealth() {
    let url = `/health`

    api.get(url)
        .then(response => {
            if (response.status === 401) {
                localStorage.removeItem('authToken');
            }
        })
        .catch(e => {
            localStorage.removeItem('authToken');
        })
}

checkServerHealth()
const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <AuthProvider>
    <Router>
      <Gads />
    </Router>
  </AuthProvider>
);

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();
