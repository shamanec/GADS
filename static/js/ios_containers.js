document.getElementById("container-logs-button").addEventListener('click', () => {
    var button = event.target
    var container_id = button.value

    getContainerLogs(container_id)
})

document.getElementById("appium-logs-button").addEventListener('click', () => {
    var button = event.target
    var udid = button.value

    getDeviceLogs(udid, "appium-logs")
})

document.getElementById("wda-logs-button").addEventListener('click', () => {
    var button = event.target
    var udid = button.value
    
    getDeviceLogs(udid, "wda-logs")
})

document.getElementById("wda-sync-logs-button").addEventListener('click', () => {
    var button = event.target
    var udid = button.value

    getDeviceLogs(udid, "wda-sync")
})

document.getElementById("restart-container-button").addEventListener('click', () => {
    var button = event.target
    var container_id = button.value

    containerAction(container_id, "restart")
})

document.getElementById("remove-container-button").addEventListener('click', () => {
    var button = event.target
    var container_id = button.value

    containerAction(container_id, "remove")
})

document.getElementById("refresh-logs-button").addEventListener('click', () => {
    var button = event.target
    var url = button.value

    refreshLogs(url)
})

/* Restart or remove a device container */
function containerAction(container_id, action) {

    /* Show loading indicator until response is returned */
    $('#loading').css("visibility", "visible");

    /* Call the endpoint that will restart the selected container */
    $.ajax({
        dataType: 'JSON',
        contentType: 'application/json',
        async: false,
        type: "POST",
        url: "/containers/" + container_id + "/" + action,
        success: function (data) {
            $('#loading').css("visibility", "hidden");
            swal("Restart container", data.message, "info")
                .then(() => {
                    location.reload();
                });
        },
        error: function (data) {
            $('#loading').css("visibility", "hidden");
            swal("Restart container", data.error_message, "error")
                .then(() => {
                    location.reload();
                });
        }
    });
}

function getDeviceLogs(udid, log_type) {
    var url = "/device-logs/" + log_type + "/" + udid
    var refreshButton = document.getElementById("refresh-logs-button")
    refreshButton.value = url
    /* Call the endpoint that will get the chosen logs */
    $.ajax({
        dataType: 'JSON',
        contentType: 'application/json',
        async: false,
        type: "GET",
        url: "/device-logs/" + log_type + "/" + udid,
        success: function (data) {
            showInfoModal(data.message)
        }
    });
}

function getContainerLogs(container_id) {
    var url = "/containers/" + container_id + "/logs"
    var refreshButton = document.getElementById("refresh-logs-button")
    refreshButton.value = url
    /* Call the endpoint that will get the container logs */
    $.ajax({
        dataType: 'JSON',
        contentType: 'application/json',
        async: false,
        type: "GET",
        url: "/containers/" + container_id + "/logs",
        success: function (data) {
            showInfoModal(data.message)
        },
        error: function (data) {
            showInfoModal(data.message)
        }
    });
}

/* Show info modal with provided text */
function showInfoModal(modalText) {
    /* Get the modal element */
    var modal = document.getElementById("deviceLogsModal")

    /* Get the close button */
    var span = document.getElementsByClassName("close")[0]

    /* Set the modal text */
    $('.modal-body').html(modalText)

    /* Display the modal blocking interaction */
    modal.style.display = "block";

    /* Close the modal if you click on close button */
    span.onclick = function () {
        modal.style.display = "none";
    }

    /* Close the modal if you click anywhere outside the modal */
    window.onclick = function (event) {
        if (event.target == modal) {
            modal.style.display = "none";
        }
    }
}

// Dynamically update the logs inside the modal without reloading
function refreshLogs(url) {
    var modalBody = document.getElementsByClassName("modal-body")[0]
    $.ajax({
        dataType: 'JSON',
        contentType: 'application/json',
        async: false,
        type: "GET",
        url: url,
        success: function (data) {
            /* Set the modal text */
            $('.modal-body').html(data.message)

            /* Scroll to the bottom of the logs on refresh */
            modalBody.scrollTop = modalBody.scrollHeight;
        }
    });
}

window.addEventListener("DOMContentLoaded", function () {
    $('.container-status-cells').each(function (i) {
        if (this.textContent.indexOf('Up') > -1) {
            this.style.backgroundColor = "#4CAF50";
        } else {
            this.style.backgroundColor = "#fcba03";
        }
    });
});