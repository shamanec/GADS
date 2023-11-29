import Swal from "sweetalert2";

export default function ShowFailedSessionAlert(deviceURL) {

    const swalWithBootstrapButtons = Swal.mixin({
        customClass: {
            confirmButton: 'btn btn-success',
            cancelButton: 'btn btn-danger'
        },
        buttonsStyling: true
    })

    swalWithBootstrapButtons.fire({
        title: 'Session lost',
        text: "Do you want to refresh or go back to devices?",
        icon: 'warning',
        showCancelButton: true,
        confirmButtonText: 'Refresh session',
        cancelButtonText: 'Back to devices',
        reverseButtons: true
    }).then((result) => {
        if (result.isConfirmed) {
            refreshSession(deviceURL)
        } else if (
            result.dismiss === Swal.DismissReason.cancel
        ) {
            window.location.href = "/devices";
        }
    })
}

async function refreshSession(deviceURL) {
    let healthURL = `${deviceURL}/health`
    await fetch(healthURL, {
        method: 'GET'
    }).catch(() => {
        window.location.href = "/devices";
    })
}

