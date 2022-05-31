$("#wda-upload-form").submit(function (e) {
    e.preventDefault();
    var formData = new FormData();
    formData.append('file', $('#wda-input-file')[0].files[0]);

    $.ajax({
        url: '/configuration/upload-wda',
        type: 'POST',
        data: formData,
        processData: false,  // tell jQuery not to process the data
        contentType: false,  // tell jQuery not to set contentType
        success: function (data) {
            swal("Upload WDA", data, "success")
            .then(() => {
                location.reload();
            });
        },
        error: function (data) {
            swal("Upload WDA", data, "error")
            .then(() => {
                location.reload();
            });
        }
    });
});

$("#app-upload-form").submit(function (e) {
    e.preventDefault();
    var formData = new FormData();
    formData.append('file', $('#app-input-file')[0].files[0]);

    $.ajax({
        url: '/configuration/upload-app',
        type: 'POST',
        data: formData,
        processData: false,  // tell jQuery not to process the data
        contentType: false,  // tell jQuery not to set contentType
        success: function (data) {
            console.log(data);
            swal("Upload app", data, "success")
            .then(() => {
                location.reload();
            });
        },
        error: function (data) {
            console.log(data);
            swal("Upload app", data, "error")
            .then(() => {
                location.reload();
            });
        }
    });
});

/* Show info modal with provided text */
function showConfigModal() {
    /* Get the modal element */
    var modal = document.getElementById("configuration-modal")

    /* Get the close button */
    var span = document.getElementsByClassName("close")[0]

    $("#modal-body").load("/static/project_config_submission_form.html");

    /* Display the modal blocking interaction */
    modal.style.display = "block";

    /* Close the modal if you click anywhere outside the modal */
    window.onclick = function (event) {
        if (event.target == modal) {
            modal.style.display = "none";
        }
    }
};

function buildImage() {
    $.ajax({
        async: true,
        type: "POST",
        url: "/configuration/build-image",
        success: function () {
            swal("Build image", "Started building docker image, this could take a while. Refresh the page occassionaly to get the image status.", "info");
        }
    });
}

function removeImage() {
    /* Show loading indicator until response is returned */
    $('#loading').css("visibility", "visible");

    /* Call the endpoint that will restart the selected container */
    $.ajax({
        dataType: 'json',
        async: true,
        type: "POST",
        url: "/configuration/remove-image",
        success: function (data) {
            swal("Remove image", data.message, "success")
            .then(() => {
                location.reload();
            });
        },
        error: function (data) {
            swal("Remove image", JSON.parse(data.responseText).error_message, "error")
            .then(() => {
                location.reload();
            });
        }
    });

    /* Hide the loading after response is returned */
    $('#loading').css("visibility", "hidden");
}

function removeUdevListener() {
    var sudoPasswordStatus = document.getElementById("sudo-password-cell").getAttribute("value")
    if (sudoPasswordStatus === "false") {
        swal("Warning", "Elevated permissions are needed to perform this action. Please set your user sudo password in the '.config.yaml' file or directly through the UI.", "info")
        return
    }
    /* Show loading indicator until response is returned */
    $('#loading').css("visibility", "visible");

    /* Call the endpoint that will restart the selected container */
    $.ajax({
        dataType: false,
        async: true,
        type: "POST",
        url: "/configuration/remove-udev-listener",
        success: function (data) {
            swal("Remove listener", "Succesfully removed iOS udev listener", "success")
            .then(() => {
                location.reload();
            });
        },
        error: function (data) {
            swal("Remove listener", JSON.stringify(data), "error")
            .then(() => {
                location.reload();
            });
        }
    });

    /* Hide the loading after response is returned */
    $('#loading').css("visibility", "hidden");
}

function setupUdevListener() {
    var sudoPasswordStatus = document.getElementById("sudo-password-cell").getAttribute("value")
    if (sudoPasswordStatus === "false") {
        swal("Warning", "Elevated permissions are needed to perform this action. Please set your user sudo password in the '.config.yaml' file or directly through the UI.", "info")
        return
    }
    var imageStatus = document.getElementById("image-status-cell").getAttribute("value")
    if (imageStatus === "is-not-available") {
        swal("Warning", "The 'ios-appium' Docker image is not available, listener will not be started.", "error")
        return
    }
    $('#loading').css("visibility", "visible");

    $.ajax({
        dataType: false,
        async: true,
        type: "POST",
        url: "/configuration/setup-udev-listener",
        success: function (data) {
            swal("Setup ios listener", data, "success")
            .then(() => {
                location.reload();
            });
        },
        error: function (data) {
            swal("Setup ios listener", JSON.stringify(data), "error")
            .then(() => {
                location.reload();
            });
        }
    });

    $('#loading').css("visibility", "hidden");
}

function showSudoPasswordInput() {

    var modal = document.getElementById("sudo-password-input-modal")

    /* Display the modal blocking interaction */
    modal.style.display = "block";

    /* Close the modal if you click anywhere outside the modal */
    window.onclick = function (event) {
        if (event.target == modal) {
            modal.style.display = "none";
        }
    }
}

function setSudoPassword() {
    var modal = document.getElementById("sudo-password-input-modal")
    var sudo_password = $("#sudo-password-input").val();
    if (sudo_password === "") {
        swal("Info message", "Please provide a non-empty value for the sudo password.", "error")
        return
    }
    // Send a request to register the device with the respective selected device UDID
    $.ajax({
        dataType: 'JSON',
        contentType: 'application/json',
        async: true,
        type: "PUT",
        data: JSON.stringify({ "sudo_password": sudo_password }),
        url: "/configuration/set-sudo-password",
        success: function (data) {
            modal.style.display = "none";
            swal("Set sudo password", data.message, "info")
            .then(() => {
                location.reload();
            });
        },
        error: function (data) {
            modal.style.display = "none";
            swal(JSON.parse(data.responseText).event, JSON.parse(data.responseText).error_message, "error")
            .then(() => {
                location.reload();
            });
        }
    });
}

function showWDAUploadForm() {
    var modal = document.getElementById("upload-wda-modal")

    // Clear the file input upon loading the modal
    $("#wda-input-file").val('');

    /* Display the modal blocking interaction */
    modal.style.display = "block";

    /* Close the modal if you click anywhere outside the modal */
    window.onclick = function (event) {
        if (event.target == modal) {
            modal.style.display = "none";
        }
    }
}

function showAppFileUploadForm() {
    var modal = document.getElementById("upload-app-modal")

    // Clear the file input upon loading the modal
    $("#app-input-file").val('');

    /* Display the modal blocking interaction */
    modal.style.display = "block";

    /* Close the modal if you click anywhere outside the modal */
    window.onclick = function (event) {
        if (event.target == modal) {
            modal.style.display = "none";
        }
    }
}

function updateContainers() {
    $.ajax({
        async: true,
        type: "POST",
        url: "/ios-containers/update",
        success: function () {
            swal("Containers update", "Started iOS containers update", "info")
        }
    });
}

function enableDisableSubmit(dropdownObj) {
    // Check the currently selected option
    // If it is the default one disable the Add button else enable it
    if (dropdownObj.options[dropdownObj.selectedIndex].text === "Choose device") {
        $('#add-device-button').prop('disabled', true);
    } else {
        $('#add-device-button').prop('disabled', false);
    }
}

function notImplemented() {
    swal("Not implemented", "Not implemented", "info")
}

window.addEventListener("DOMContentLoaded", function () {
    setWDAStatusCellBackground()
    setImageStatusCellBackground()
    setUdevIOSListenerCellBackground()
    setSudoPasswordCellBackground()
});

function setWDAStatusCellBackground() {
    let statusCell = document.getElementById("wda-status-cell")
    if (statusCell.textContent === "true") {
        statusCell.style.backgroundColor = "#4CAF50"
        statusCell.setAttribute("value", "true")
    } else {
        statusCell.style.backgroundColor = "#eb4f34"
        statusCell.setAttribute("value", "false")
    }
}

function setImageStatusCellBackground() {
    let statusCell = document.getElementById("image-status-cell");
    if (statusCell.textContent === "Image available") {
        statusCell.style.backgroundColor = "#4CAF50"
        statusCell.setAttribute("value", "is-available")
    } else if (statusCell.textContent === "Image not available") {
        statusCell.style.backgroundColor = "#eb4f34"
        statusCell.setAttribute("value", "is-not-available")
    } else {
        statusCell.style.backgroundColor = "#ebe134"
        statusCell.setAttribute("value", "undefined")
    }
}

function setUdevIOSListenerCellBackground() {
    let statusCell = document.getElementById("udev-udev-listener-status-cell");
    if (statusCell.textContent === "Udev rules set.") {
        statusCell.style.backgroundColor = "#4CAF50"
        statusCell.setAttribute("value", "is-running")
    } else if (statusCell.textContent === "Udev rules not set.") {
        statusCell.style.backgroundColor = "#eb4f34"
        statusCell.setAttribute("value", "is-not-running")
    }
}

function setSudoPasswordCellBackground() {
    let statusCell = document.getElementById("sudo-password-cell")
    if (statusCell.textContent === "true") {
        statusCell.style.backgroundColor = "#4CAF50"
        statusCell.setAttribute("value", "true")
    } else {
        statusCell.style.backgroundColor = "#eb4f34"
        statusCell.setAttribute("value", "false")
    }
}