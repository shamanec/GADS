package proxy

import (
	"GADS/device"
	"io"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

// This handler accepts device related commands and proxies them to the respective device provider
// So we don't call the provider directly
func ProxyHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	path := vars["path"]
	udid := vars["udid"]

	device := device.GetDeviceByUDID(udid)

	// Replace this URL with your provider server's base URL
	providerBaseURL := "http://" + device.Host + ":10001"

	providerURL, err := url.Parse(providerBaseURL + "/device/" + udid + "/" + path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Forward the request to the provider server
	req, err := http.NewRequest(r.Method, providerURL.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy the response from the provider server to the client
	for k, v := range resp.Header {
		w.Header().Set(k, v[0])
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
