import Info from "./Info"

export default function ListProvider({ info, handleSelectProvider }) {
    return (
        <div>
            <Info handleSelectProvider={handleSelectProvider} info={info}></Info>
        </div>
    )
}