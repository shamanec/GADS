window.addEventListener("DOMContentLoaded", function () {
    addDeviceSelectionListeners()
});

function addDeviceSelectionListeners() {
    document.querySelectorAll(".ios-device-use").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
            use_device("ios", udid)
        })
    })

    document.querySelectorAll(".ios-device-info").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
        })
    })

    document.querySelectorAll(".android-device-use").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
            use_device("android", udid)
        })
    })

    document.querySelectorAll(".android-device-info").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
        })
    })
}

function use_device(type, udid) {

    /* Show loading indicator until response is returned */
    $('#loading').css("visibility", "visible");

    // Build the url for the respective action
    var url = "/devices/device-control-info"

    var finalJson = JSON.stringify({
        "type": type,
        "udid": udid
    })

    /* Call the endpoint that will restart/remove the selected container */
    $.ajax({
        contentType: 'text/html',
        async: false,
        data: finalJson,
        type: "POST",
        url: url,
        success: function (data) {
            document.getElementById('mega-div').innerHTML = ""
            console.log("koleo")
            $('#loading').css("visibility", "hidden");
            document.getElementById('mega-div').innerHTML = data
        },
        error: function (data) {

        }
    });
}

function reloadDevices() {
    /* Call the endpoint that will restart/remove the selected container */
    $.ajax({
        contentType: 'text/html',
        async: false,
        type: "GET",
        url: "http://localhost:10000/devices/reload-available-devices",
        success: function (data) {
            document.getElementById('mega-div').innerHTML = ""
            document.getElementById('mega-div').innerHTML = data
            addDeviceSelectionListeners()
        }
    });
}