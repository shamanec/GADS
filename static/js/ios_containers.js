// Add listeners to each container info button
// We have it as a function so we can re-add the listeners each time the table updates
function addListeners() {
    document.querySelectorAll("#container-logs-button").forEach((e) => {
        e.addEventListener("click", (button) => {
            var container_id = button.target.value
            getContainerLogs(container_id)
        })
    })

    document.querySelectorAll("#appium-logs-button").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
            getDeviceLogs(udid, "appium-logs")
        })
    })

    document.querySelectorAll("#wda-logs-button").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
            getDeviceLogs(udid, "wda-logs")
        })
    })

    document.querySelectorAll("#wda-sync-logs-button").forEach((e) => {
        e.addEventListener("click", (button) => {
            var udid = button.target.value
            getDeviceLogs(udid, "wda-sync")
        })
    })

    document.querySelectorAll("#restart-container-button").forEach((e) => {
        e.addEventListener("click", (button) => {
            var container_id = button.target.value
            containerAction(container_id, "restart")
        })
    })

    document.querySelectorAll("#remove-container-button").forEach((e) => {
        e.addEventListener("click", (button) => {
            var container_id = button.target.value
            containerAction(container_id, "remove")
        })
    })
}

// Add an event listener for the refresh button of the logs
// It is not part of the table, but of the logs modal
// So we don't need to have it in the addListeners() function
document.getElementById("refresh-logs-button").addEventListener('click', (button) => {
    var url = button.target.value
    refreshLogs(url)
})

// Refresh the containers table on the page each 5 seconds
function refreshContainers() {
    updateTimer()
    // Set the background colour of the container status cells
    setStatusColour()

    // Call the refresh-ios-containers endpoint
    // And get updated html table for the containers
    $.ajax({
        contentType: 'text/html',
        type: "GET",
        async: false,
        url: "/refresh-ios-containers",
        success: function (data) {
            // Update the containers table with the new table data
            document.getElementById('containers-table').innerHTML = data

            // Set the background colour of the container status cells again
            setStatusColour()
        },
        error: function (data) {
        }
    });

    // Re-add the listeners for the info buttons after updating the table
    addListeners()

    // Schedule the next refresh
    setTimeout(refreshContainers, 5000);
}

// Set the background colour of the container status cells
// Green for Up and yellow for all others
function setStatusColour() {
    $('.container-status-cells').each(function (i) {
        if (this.textContent.indexOf('Up') > -1) {
            this.style.backgroundColor = "#4CAF50";
        } else {
            this.style.backgroundColor = "#fcba03";
        }
    });
}

/* Restart or remove a device container */
function containerAction(container_id, action) {

    /* Show loading indicator until response is returned */
    $('#loading').css("visibility", "visible");

    // Build the url for the respective action
    var url = "/containers/" + container_id + "/" + action

    /* Call the endpoint that will restart/remove the selected container */
    $.ajax({
        dataType: 'JSON',
        contentType: 'application/json',
        async: false,
        type: "POST",
        url: url,
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

function updateTimer() {
    var timeleft = 5;
    var downloadTimer = setInterval(function(){
    timeleft--;
    document.getElementById("countdown-timer").textContent = "Refreshing in " + timeleft + "...";
    if(timeleft <= 0)
        clearInterval(downloadTimer);
    },1000);
}

// Get the logs for a device
function getDeviceLogs(udid, log_type) {
    // Build the url for the respective log type
    var url = "/device-logs/" + log_type + "/" + udid

    // Update the logs modal refresh button value to the same url
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

// Get the logs for a device contaienr
function getContainerLogs(container_id) {
    // Build the url for the respective container logs
    var url = "/containers/" + container_id + "/logs"

    // Update the logs modal refresh button value to the same url
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
    var modal = document.getElementById("device-logs-modal")

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

// On page load add the listeners and start the refresh containers job
window.addEventListener("DOMContentLoaded", function () {
    addListeners()
    refreshContainers()
});