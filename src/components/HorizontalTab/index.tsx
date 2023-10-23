import { SetStateAction } from 'react'

import styles from './styles.module.scss'

interface HorizontalTabItem {
    name: string,
    id: number
}

interface HorizontalTabProps {
    option: number,
    setOption: (id: number) => void,
    items: HorizontalTabItem[]
}

export function HorizontalTab({ option, setOption, items }: HorizontalTabProps) {
    const changeOption = (id: number) => setOption(id)

    return(
        <div className={styles.horizontalContainer}>
            <div className={styles.horizontalHeader}>
                {
                    items.map(i => {
                        return (
                            <button
                                key={i.id}
                                className={`${styles.item} ${option === i.id && styles.active}`}
                                onClick={() => changeOption(i.id)}
                            >
                                {i.name}
                            </button>
                        )
                    })
                }
            </div>
        </div>
    )
}
