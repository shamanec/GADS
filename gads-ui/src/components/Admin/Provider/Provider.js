
export default function Provider({ info }) {
    return (
        <div>
            <div>{info.name}</div>
            <div>{info.devices}</div>
            <div>{info.host_address}</div>
        </div>
    )
}