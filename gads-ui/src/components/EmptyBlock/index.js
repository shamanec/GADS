import styles from './styles.module.scss'

export function EmptyBlock({ emptyText, emptySupporting }) {
    return(
        <div className={styles.empty}>
            <img src='./images/empty-illustration.svg' alt='empty illustration' />
            <h2>{ emptyText }</h2>
            <span>{ emptySupporting }</span>
        </div>
    )
}